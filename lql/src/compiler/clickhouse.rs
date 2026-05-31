use crate::ast::*;
use crate::error::LqlError;
use crate::functions::duration_to_clickhouse_interval;
use crate::schema::Schema;

/// Compile a parsed LQL pipeline into ClickHouse SQL.
pub fn compile(pipeline: &Pipeline, schema: &Schema) -> Result<String, LqlError> {
    let mut table = schema.table.clone();
    let mut where_clauses = Vec::new();
    let mut select_cols = Vec::new();
    let mut group_by = Vec::new();
    let mut order_by = None;
    let mut limit = None;
    let mut extended_columns: Vec<String> = Vec::new();

    for stmt in &pipeline.statements {
        match stmt {
            Statement::From(source) => {
                table = source.table_name().to_string();
            }
            Statement::Where(expr) => {
                where_clauses.push(compile_expr_with_aliases(expr, schema, &extended_columns)?);
            }
            Statement::Summarize { aggregations, by } => {
                group_by = by
                    .iter()
                    .map(|e| compile_expr(e, schema))
                    .collect::<Result<Vec<_>, _>>()?;
                select_cols = aggregations
                    .iter()
                    .map(|a| compile_agg(a, schema))
                    .collect::<Result<Vec<_>, _>>()?;
            }
            Statement::Sort { field, order } => {
                order_by = Some(format!("{} {}", compile_expr(field, schema)?, order));
            }
            Statement::Limit(n) => {
                limit = Some(*n);
            }
            Statement::Project(fields) => {
                select_cols = fields
                    .iter()
                    .map(|e| compile_expr(e, schema))
                    .collect::<Result<Vec<_>, _>>()?;
            }
            Statement::Extend { name, expr } => {
                select_cols.push(format!(
                    "{} AS {}",
                    compile_expr_with_aliases(expr, schema, &extended_columns)?,
                    escape_ident(name)
                ));
                extended_columns.push(name.clone());
            }
            Statement::Top { count, by, order } => {
                let by_cols = by
                    .iter()
                    .map(|e| compile_expr(e, schema))
                    .collect::<Result<Vec<_>, _>>()?;
                order_by = Some(format!("{} {}", by_cols.join(", "), order));
                limit = Some(*count);
            }
            Statement::Timeseries { interval } => {
                let bucket = format!("toStartOf{}({})", interval_unit(interval), schema.ts_column);
                select_cols = vec![bucket.clone(), "COUNT(*) AS count".to_string()];
                group_by = vec![bucket];
                order_by = Some(format!("{} ASC", schema.ts_column));
            }
        }
    }

    let mut sql = String::new();
    if select_cols.is_empty() {
        sql.push_str("SELECT *");
    } else {
        sql.push_str(&format!("SELECT {}", select_cols.join(", ")));
    }
    sql.push_str(&format!(" FROM {}", escape_ident(&table)));
    if !where_clauses.is_empty() {
        sql.push_str(&format!(" WHERE {}", where_clauses.join(" AND ")));
    }
    if !group_by.is_empty() {
        sql.push_str(&format!(" GROUP BY {}", group_by.join(", ")));
    }
    if let Some(ref order) = order_by {
        sql.push_str(&format!(" ORDER BY {}", order));
    }
    if let Some(limit) = limit {
        sql.push_str(&format!(" LIMIT {}", limit));
    }
    Ok(sql)
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
            if schema.has_field(name) || aliases.iter().any(|a| a == name) {
                Ok(escape_ident(name))
            } else {
                Ok(format!("JSONExtractString(raw, '{}')", name))
            }
        }
        Expr::Literal(lit) => compile_literal(lit),
        Expr::BinaryOp { left, op, right } => {
            let l = compile_expr_with_aliases(left, schema, aliases)?;
            let r = compile_expr_with_aliases(right, schema, aliases)?;
            match op {
                BinOp::Like => Ok(format!("{} LIKE {}", l, r)),
                BinOp::NotLike => Ok(format!("{} NOT LIKE {}", l, r)),
                BinOp::Contains | BinOp::Has => Ok(format!(
                    "{} LIKE '%{}%'",
                    l,
                    escape_like_pattern(&strip_quotes(&r))
                )),
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
        Literal::Bool(b) => Ok(if *b { "true" } else { "false" }.to_string()),
        Literal::Null => Ok("NULL".to_string()),
        Literal::Duration(d) => Ok(format!("now() - {}", duration_to_clickhouse_interval(d))),
    }
}

fn compile_agg(agg: &AggExpr, schema: &Schema) -> Result<String, LqlError> {
    let func_str = match &agg.function {
        AggFunction::Count => {
            if let Some(arg) = &agg.arg {
                format!("count({})", compile_expr(arg, schema)?)
            } else {
                "count(*)".to_string()
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
                AggFunction::Sum => format!("sum({})", compiled),
                AggFunction::Avg => format!("avg({})", compiled),
                AggFunction::Min => format!("min({})", compiled),
                AggFunction::Max => format!("max({})", compiled),
                AggFunction::DCount => format!("uniq({})", compiled),
                AggFunction::First => format!("any({})", compiled),
                AggFunction::Last => format!("anyLast({})", compiled),
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
            format!("quantile({})({})", pct, compiled)
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
                Ok(format!("now() - {}", duration_to_clickhouse_interval(d)))
            } else {
                Err(LqlError::Compile {
                    message: "ago() requires a duration argument".to_string(),
                    span: None,
                })
            }
        }
        "now" => Ok("now()".to_string()),
        "coalesce" => Ok(format!("ifNull({})", compiled_args.join(", "))),
        "isempty" => Ok(format!(
            "({} = '' OR {} IS NULL)",
            compiled_args[0], compiled_args[0]
        )),
        "isnotempty" => Ok(format!(
            "({} != '' AND {} IS NOT NULL)",
            compiled_args[0], compiled_args[0]
        )),
        "strlen" => Ok(format!("length({})", compiled_args[0])),
        "tolower" => Ok(format!("lower({})", compiled_args[0])),
        "toupper" => Ok(format!("upper({})", compiled_args[0])),
        "trim" => Ok(format!("trim({})", compiled_args[0])),
        "to_string" => Ok(format!("toString({})", compiled_args[0])),
        "to_int" => Ok(format!("toInt64({})", compiled_args[0])),
        "to_float" => Ok(format!("toFloat64({})", compiled_args[0])),
        "abs" => Ok(format!("abs({})", compiled_args[0])),
        "round" => {
            if compiled_args.len() == 2 {
                Ok(format!("round({}, {})", compiled_args[0], compiled_args[1]))
            } else {
                Ok(format!("round({})", compiled_args[0]))
            }
        }
        "floor" => Ok(format!("floor({})", compiled_args[0])),
        "ceil" => Ok(format!("ceil({})", compiled_args[0])),
        "log" => Ok(format!("log({})", compiled_args[0])),
        "sqrt" => Ok(format!("sqrt({})", compiled_args[0])),
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

fn interval_unit(d: &Duration) -> &'static str {
    match d.unit {
        DurationUnit::Milliseconds => "Millisecond",
        DurationUnit::Seconds => "Second",
        DurationUnit::Minutes => "Minute",
        DurationUnit::Hours => "Hour",
        DurationUnit::Days => "Day",
        DurationUnit::Weeks => "Week",
    }
}
