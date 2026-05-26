use crate::ast::*;
use crate::error::LqlError;
use crate::schema::Schema;

/// Validate a pipeline against the schema.
/// Checks that referenced fields exist and function names are valid.
pub fn validate(pipeline: &Pipeline, schema: &Schema) -> Result<(), LqlError> {
    for stmt in &pipeline.statements {
        match stmt {
            Statement::Where(expr) => validate_expr(expr, schema)?,
            Statement::Summarize { aggregations, by } => {
                for agg in aggregations {
                    if let Some(arg) = &agg.arg {
                        validate_expr(arg, schema)?;
                    }
                }
                for expr in by {
                    validate_expr(expr, schema)?;
                }
            }
            Statement::Sort { field, .. } => validate_expr(field, schema)?,
            Statement::Project(fields) => {
                for expr in fields {
                    validate_expr(expr, schema)?;
                }
            }
            Statement::Extend { expr, .. } => validate_expr(expr, schema)?,
            Statement::Top { by, .. } => {
                for expr in by {
                    validate_expr(expr, schema)?;
                }
            }
            _ => {}
        }
    }
    Ok(())
}

fn validate_expr(expr: &Expr, schema: &Schema) -> Result<(), LqlError> {
    match expr {
        Expr::Column(name) => {
            // Allow dotted paths (user.name, tenant.id, etc.) and virtual fields
            let base = name.split('.').next().unwrap_or(name);
            let virtual_fields = ["count", "timestamp", "ts", "raw"];
            let nested_roots = ["user", "tenant", "http", "resource", "attrs", "metadata"];
            if !schema.has_field(base)
                && !schema.has_field(name)
                && !virtual_fields.contains(&base)
                && !nested_roots.contains(&base)
            {
                return Err(LqlError::UnknownField {
                    name: name.clone(),
                    span: crate::error::Span::new(0, 0),
                });
            }
        }
        Expr::BinaryOp { left, right, .. } => {
            validate_expr(left, schema)?;
            validate_expr(right, schema)?;
        }
        Expr::UnaryOp { expr, .. } => validate_expr(expr, schema)?,
        Expr::Function { name, args } => {
            if !crate::functions::is_builtin_function(name) {
                return Err(LqlError::UnknownFunction {
                    name: name.clone(),
                    span: crate::error::Span::new(0, 0),
                });
            }
            for arg in args {
                validate_expr(arg, schema)?;
            }
        }
        Expr::InList { expr, values } => {
            validate_expr(expr, schema)?;
            for v in values {
                validate_expr(v, schema)?;
            }
        }
        _ => {}
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::lexer::Lexer;
    use crate::parser::Parser;

    fn validate_query(input: &str) -> Result<(), LqlError> {
        let tokens = Lexer::new(input).tokenize().unwrap();
        let pipeline = Parser::new(tokens).parse().unwrap();
        let schema = Schema::duckdb_default();
        validate(&pipeline, &schema)
    }

    #[test]
    fn valid_field_passes() {
        assert!(validate_query(r#"from events | where service = "checkout""#).is_ok());
    }

    #[test]
    fn unknown_field_fails() {
        assert!(validate_query(r#"from events | where nonexistent = "x""#).is_err());
    }

    #[test]
    fn nested_field_passes() {
        assert!(validate_query(r#"from events | where user.name = "x""#).is_ok());
    }
}
