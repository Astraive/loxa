use crate::ast::*;
use crate::error::LqlError;
use crate::functions::duration_to_duckdb_interval;
use crate::schema::Schema;

/// Compile a parsed LQL pipeline into DuckDB SQL.
pub fn compile(pipeline: &Pipeline, schema: &Schema) -> Result<String, LqlError> {
    let mut ctx = CompileCtx::new(schema);
    let mut sql = String::new();

    for stmt in &pipeline.statements {
        match stmt {
            Statement::From(source) => {
                ctx.table = source.table_name().to_string();
            }
            Statement::Where(expr) => {
                ctx.where_clauses.push(compile_expr_with_aliases(
                    expr,
                    schema,
                    &ctx.extended_columns,
                )?);
            }
            Statement::Summarize { aggregations, by } => {
                ctx.group_by = by
                    .iter()
                    .map(|e| compile_expr(e, schema))
                    .collect::<Result<Vec<_>, _>>()?;
                ctx.select_cols = aggregations
                    .iter()
                    .map(|a| compile_agg(a, schema))
                    .collect::<Result<Vec<_>, _>>()?;
            }
            Statement::Sort { field, order } => {
                ctx.order_by = Some(format!("{} {}", compile_expr(field, schema)?, order));
            }
            Statement::Limit(n) => {
                ctx.limit = Some(*n);
            }
            Statement::Project(fields) => {
                ctx.select_cols = fields
                    .iter()
                    .map(|e| compile_expr(e, schema))
                    .collect::<Result<Vec<_>, _>>()?;
            }
            Statement::Extend { name, expr } => {
                ctx.select_cols.push(format!(
                    "{} AS {}",
                    compile_expr_with_aliases(expr, schema, &ctx.extended_columns)?,
                    escape_ident(name)
                ));
                ctx.extended_columns.push(name.clone());
            }
            Statement::Top { count, by, order } => {
                let by_cols = by
                    .iter()
                    .map(|e| compile_expr(e, schema))
                    .collect::<Result<Vec<_>, _>>()?;
                ctx.order_by = Some(format!("{} {}", by_cols.join(", "), order));
                ctx.limit = Some(*count);
            }
            Statement::Timeseries { interval } => {
                let bucket = format!(
                    "date_trunc('{}', {})",
                    interval_unit(interval),
                    schema.ts_column
                );
                ctx.select_cols = vec![bucket.clone(), "COUNT(*) AS count".to_string()];
                ctx.group_by = vec![bucket];
                ctx.order_by = Some(format!("{} ASC", schema.ts_column));
            }
        }
    }

    // Build SELECT
    if ctx.select_cols.is_empty() {
        sql.push_str("SELECT *");
    } else {
        sql.push_str(&format!("SELECT {}", ctx.select_cols.join(", ")));
    }

    // FROM
    sql.push_str(&format!(" FROM {}", escape_ident(&ctx.table)));

    // WHERE
    if !ctx.where_clauses.is_empty() {
        sql.push_str(&format!(" WHERE {}", ctx.where_clauses.join(" AND ")));
    }

    // GROUP BY
    if !ctx.group_by.is_empty() {
        sql.push_str(&format!(" GROUP BY {}", ctx.group_by.join(", ")));
    }

    // ORDER BY
    if let Some(ref order) = ctx.order_by {
        sql.push_str(&format!(" ORDER BY {}", order));
    }

    // LIMIT
    if let Some(limit) = ctx.limit {
        sql.push_str(&format!(" LIMIT {}", limit));
    }

    Ok(sql)
}

struct CompileCtx {
    table: String,
    where_clauses: Vec<String>,
    select_cols: Vec<String>,
    group_by: Vec<String>,
    order_by: Option<String>,
    limit: Option<usize>,
    extended_columns: Vec<String>,
}

impl CompileCtx {
    fn new(schema: &Schema) -> Self {
        Self {
            table: schema.table.clone(),
            where_clauses: Vec::new(),
            select_cols: Vec::new(),
            group_by: Vec::new(),
            order_by: None,
            limit: None,
            extended_columns: Vec::new(),
        }
    }
}

fn compile_expr(expr: &Expr, schema: &Schema) -> Result<String, LqlError> {
    compile_expr_with_aliases(expr, schema, &[])
}

fn compile_expr_with_aliases(
    expr: &Expr,
    schema: &Schema,
    aliases: &[String],
) -> Result<String, LqlError> {
    match expr {
        Expr::Column(name) => {
            // Map dot-paths to json_extract
            if name.contains('.') {
                Ok(format!(
                    "json_extract_string({}, '$.{}')",
                    schema.raw_column, name
                ))
            } else if schema.has_field(name)
                || name == "count"
                || name == "timestamp"
                || name == "ts"
                || aliases.iter().any(|a| a == name)
            {
                Ok(escape_ident(name))
            } else {
                Ok(format!(
                    "json_extract_string({}, '$.{}')",
                    schema.raw_column, name
                ))
            }
        }
        Expr::Literal(lit) => compile_literal(lit),
        Expr::BinaryOp { left, op, right } => {
            let l = compile_expr_with_aliases(left, schema, aliases)?;
            let r = compile_expr_with_aliases(right, schema, aliases)?;
            match op {
                BinOp::Like | BinOp::NotLike | BinOp::Contains | BinOp::Has => {
                    let op_str = match op {
                        BinOp::Like => "LIKE",
                        BinOp::NotLike => "NOT LIKE",
                        BinOp::Contains | BinOp::Has => "LIKE",
                        _ => unreachable!(),
                    };
                    let pattern = match op {
                        BinOp::Contains | BinOp::Has => {
                            format!("%{}%", escape_like_pattern(&strip_quotes(&r)))
                        }
                        _ => r.clone(),
                    };
                    Ok(format!("{} {} '{}'", l, op_str, strip_quotes(&pattern)))
                }
                BinOp::StartsWith => Ok(format!(
                    "{} LIKE '{}%'",
                    l,
                    escape_like_pattern(&strip_quotes(&r))
                )),
                BinOp::EndsWith => Ok(format!(
                    "{} LIKE '%{}'",
                    l,
                    escape_like_pattern(&strip_quotes(&r))
                )),
                _ => Ok(format!("{} {} {}", l, op, r)),
            }
        }
        Expr::UnaryOp { op, expr } => {
            let inner = compile_expr_with_aliases(expr, schema, aliases)?;
            match op {
                UnaryOp::Not => Ok(format!("NOT ({})", inner)),
                UnaryOp::Neg => Ok(format!("(-({}))", inner)),
            }
        }
        Expr::Function { name, args } => compile_function(name, args, schema),
        Expr::InList { expr, values } => {
            let col = compile_expr_with_aliases(expr, schema, aliases)?;
            let vals: Vec<String> = values
                .iter()
                .map(|v| compile_expr_with_aliases(v, schema, aliases))
                .collect::<Result<Vec<_>, _>>()?;
            Ok(format!("{} IN ({})", col, vals.join(", ")))
        }
        Expr::Wildcard => Ok("*".to_string()),
    }
}

fn compile_literal(lit: &Literal) -> Result<String, LqlError> {
    match lit {
        Literal::String(s) => Ok(format!("'{}'", s.replace('\'', "''"))),
        Literal::Integer(n) => Ok(n.to_string()),
        Literal::Float(n) => Ok(n.to_string()),
        Literal::Bool(b) => Ok(if *b { "TRUE" } else { "FALSE" }.to_string()),
        Literal::Null => Ok("NULL".to_string()),
        Literal::Duration(d) => Ok(format!("NOW() - {}", chrono_offset(d))),
    }
}

fn compile_agg(agg: &AggExpr, schema: &Schema) -> Result<String, LqlError> {
    let func_str = match &agg.function {
        AggFunction::Count => {
            if let Some(arg) = &agg.arg {
                format!("COUNT({})", compile_expr(arg, schema)?)
            } else {
                "COUNT(*)".to_string()
            }
        }
        AggFunction::Sum
        | AggFunction::Avg
        | AggFunction::Min
        | AggFunction::Max
        | AggFunction::DCount
        | AggFunction::First
        | AggFunction::Last => {
            let arg = agg.arg.as_ref().ok_or_else(|| LqlError::Compile {
                message: format!("{:?} requires an argument", agg.function),
                span: None,
            })?;
            let compiled = compile_expr(arg, schema)?;
            match &agg.function {
                AggFunction::Sum => format!("SUM({})", compiled),
                AggFunction::Avg => format!("AVG({})", compiled),
                AggFunction::Min => format!("MIN({})", compiled),
                AggFunction::Max => format!("MAX({})", compiled),
                AggFunction::DCount => format!("COUNT(DISTINCT {})", compiled),
                AggFunction::First => format!("FIRST({})", compiled),
                AggFunction::Last => format!("LAST({})", compiled),
                _ => unreachable!(),
            }
        }
        AggFunction::P50 | AggFunction::P95 | AggFunction::P99 | AggFunction::Percentile(_) => {
            let arg = agg.arg.as_ref().ok_or_else(|| LqlError::Compile {
                message: "percentile aggregation requires an argument".to_string(),
                span: None,
            })?;
            let compiled = compile_expr(arg, schema)?;
            let pct = match &agg.function {
                AggFunction::P50 => 0.5,
                AggFunction::P95 => 0.95,
                AggFunction::P99 => 0.99,
                AggFunction::Percentile(p) => p / 100.0,
                _ => unreachable!(),
            };
            format!(
                "PERCENTILE_CONT({}) WITHIN GROUP (ORDER BY {})",
                pct, compiled
            )
        }
    };

    if let Some(alias) = &agg.alias {
        Ok(format!("{} AS {}", func_str, escape_ident(alias)))
    } else {
        Ok(func_str)
    }
}

fn compile_function(name: &str, args: &[Expr], schema: &Schema) -> Result<String, LqlError> {
    let compiled_args: Vec<String> = args
        .iter()
        .map(|a| compile_expr(a, schema))
        .collect::<Result<Vec<_>, _>>()?;

    match name.to_lowercase().as_str() {
        "ago" => {
            if let Some(Expr::Literal(Literal::Duration(d))) = args.first() {
                Ok(format!("NOW() - {}", duration_to_duckdb_interval(d)))
            } else {
                Err(LqlError::Compile {
                    message: "ago() requires a duration argument (e.g. ago(1h))".to_string(),
                    span: None,
                })
            }
        }
        "bin" => {
            if compiled_args.len() == 2 {
                Ok(format!(
                    "date_trunc({}, {})",
                    compiled_args[1], compiled_args[0]
                ))
            } else {
                Err(LqlError::Compile {
                    message: "bin() requires 2 arguments: bin(field, interval)".to_string(),
                    span: None,
                })
            }
        }
        "now" => Ok("NOW()".to_string()),
        "coalesce" => Ok(format!("COALESCE({})", compiled_args.join(", "))),
        "if" => {
            if compiled_args.len() == 3 {
                Ok(format!(
                    "CASE WHEN {} THEN {} ELSE {} END",
                    compiled_args[0], compiled_args[1], compiled_args[2]
                ))
            } else {
                Err(LqlError::Compile {
                    message: "if() requires 3 arguments: if(condition, then, else)".to_string(),
                    span: None,
                })
            }
        }
        "isempty" => Ok(format!(
            "({} IS NULL OR {} = '')",
            compiled_args[0], compiled_args[0]
        )),
        "isnotempty" => Ok(format!(
            "({} IS NOT NULL AND {} != '')",
            compiled_args[0], compiled_args[0]
        )),
        "strlen" => Ok(format!("LENGTH({})", compiled_args[0])),
        "tolower" => Ok(format!("LOWER({})", compiled_args[0])),
        "toupper" => Ok(format!("UPPER({})", compiled_args[0])),
        "trim" => Ok(format!("TRIM({})", compiled_args[0])),
        "substring" => {
            if compiled_args.len() == 3 {
                Ok(format!(
                    "SUBSTRING({}, {}, {})",
                    compiled_args[0], compiled_args[1], compiled_args[2]
                ))
            } else {
                Err(LqlError::Compile {
                    message: "substring() requires 3 arguments".to_string(),
                    span: None,
                })
            }
        }
        "typeof" => Ok(format!("TYPEOF({})", compiled_args[0])),
        "to_string" => Ok(format!("CAST({} AS VARCHAR)", compiled_args[0])),
        "to_int" => Ok(format!("CAST({} AS BIGINT)", compiled_args[0])),
        "to_float" => Ok(format!("CAST({} AS DOUBLE)", compiled_args[0])),
        "abs" => Ok(format!("ABS({})", compiled_args[0])),
        "round" => {
            if compiled_args.len() == 2 {
                Ok(format!("ROUND({}, {})", compiled_args[0], compiled_args[1]))
            } else {
                Ok(format!("ROUND({})", compiled_args[0]))
            }
        }
        "floor" => Ok(format!("FLOOR({})", compiled_args[0])),
        "ceil" => Ok(format!("CEIL({})", compiled_args[0])),
        "log" => Ok(format!("LOG({})", compiled_args[0])),
        "sqrt" => Ok(format!("SQRT({})", compiled_args[0])),
        _ => Err(LqlError::UnknownFunction {
            name: name.to_string(),
            span: crate::error::Span::new(0, 0),
        }),
    }
}

fn escape_ident(name: &str) -> String {
    if name == "*" {
        name.to_string()
    } else {
        format!("\"{}\"", name.replace('"', "\"\""))
    }
}

fn strip_quotes(s: &str) -> String {
    if s.len() >= 2
        && ((s.starts_with('\'') && s.ends_with('\'')) || (s.starts_with('"') && s.ends_with('"')))
    {
        s[1..s.len() - 1].to_string()
    } else {
        s.to_string()
    }
}

fn escape_like_pattern(s: &str) -> String {
    s.replace('\\', "\\\\")
        .replace('\'', "''")
        .replace('%', "\\%")
        .replace('_', "\\_")
}

fn chrono_offset(d: &Duration) -> String {
    use crate::functions::duration_to_duckdb_interval;
    duration_to_duckdb_interval(d)
}

fn interval_unit(d: &Duration) -> &'static str {
    match d.unit {
        DurationUnit::Milliseconds => "milliseconds",
        DurationUnit::Seconds => "second",
        DurationUnit::Minutes => "minute",
        DurationUnit::Hours => "hour",
        DurationUnit::Days => "day",
        DurationUnit::Weeks => "week",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::lexer::Lexer;
    use crate::parser::Parser;

    fn compile_query(input: &str) -> String {
        let tokens = Lexer::new(input).tokenize().unwrap();
        let pipeline = Parser::new(tokens).parse().unwrap();
        let schema = Schema::duckdb_default();
        compile(&pipeline, &schema).unwrap()
    }

    #[test]
    fn simple_select() {
        let sql = compile_query(r#"from events | where service = "checkout" | limit 10"#);
        assert!(sql.contains("WHERE"));
        assert!(sql.contains("LIMIT 10"));
    }

    #[test]
    fn summarize_count() {
        let sql = compile_query(r#"from events | summarize count() by service"#);
        assert!(sql.contains("COUNT(*)"));
        assert!(sql.contains("GROUP BY"));
    }

    #[test]
    fn where_level_error() {
        let sql = compile_query(r#"from events | where level = "error""#);
        assert!(sql.contains("WHERE"));
        assert!(sql.contains("'error'"));
    }

    #[test]
    fn sort_desc() {
        let sql = compile_query("from events | sort duration_ms desc | limit 5");
        assert!(sql.contains("ORDER BY"));
        assert!(sql.contains("DESC"));
        assert!(sql.contains("LIMIT 5"));
    }
}
