use crate::ast::*;
use crate::error::LqlError;
use crate::lexer::Token;

pub struct Parser {
    tokens: Vec<Token>,
    pos: usize,
}

impl Parser {
    pub fn new(tokens: Vec<Token>) -> Self {
        Self { tokens, pos: 0 }
    }

    pub fn parse(&mut self) -> Result<Pipeline, LqlError> {
        let mut statements = Vec::new();

        // First statement must be 'from'
        statements.push(self.parse_from()?);

        // Parse pipe-separated statements
        while self.pos < self.tokens.len() {
            self.expect_token(&Token::Pipe)?;
            statements.push(self.parse_statement()?);
        }

        Ok(Pipeline { statements })
    }

    fn parse_from(&mut self) -> Result<Statement, LqlError> {
        self.expect_keyword(&Token::From)?;
        let source = match self.current() {
            Token::Ident(name) => {
                let name = name.clone();
                self.advance();
                match name.to_lowercase().as_str() {
                    "events" => Source::Events,
                    "traces" => Source::Traces,
                    "incidents" => Source::Incidents,
                    _ => Source::Table(name),
                }
            }
            _ => return Err(self.unexpected("table name")),
        };
        Ok(Statement::From(source))
    }

    fn parse_statement(&mut self) -> Result<Statement, LqlError> {
        match self.current().clone() {
            Token::Where => {
                self.advance();
                let expr = self.parse_or_expr()?;
                Ok(Statement::Where(expr))
            }
            Token::Summarize => {
                self.advance();
                let aggregations = self.parse_agg_list()?;
                let mut by = Vec::new();
                if self.check(&Token::By) {
                    self.advance();
                    by = self.parse_expr_list()?;
                }
                Ok(Statement::Summarize { aggregations, by })
            }
            Token::Sort => {
                self.advance();
                let field = self.parse_expr()?;
                let order = if self.check(&Token::Asc) {
                    self.advance();
                    Order::Asc
                } else if self.check(&Token::Desc) {
                    self.advance();
                    Order::Desc
                } else {
                    Order::Desc // default
                };
                Ok(Statement::Sort { field, order })
            }
            Token::Limit => {
                self.advance();
                let n = self.expect_integer()?;
                if n < 0 {
                    return Err(LqlError::Compile {
                        message: "limit must be non-negative".to_string(),
                        span: None,
                    });
                }
                Ok(Statement::Limit(n as usize))
            }
            Token::Project => {
                self.advance();
                let fields = self.parse_expr_list()?;
                Ok(Statement::Project(fields))
            }
            Token::Extend => {
                self.advance();
                let name = self.expect_ident()?;
                self.expect_token(&Token::Eq)?;
                let expr = self.parse_expr()?;
                Ok(Statement::Extend { name, expr })
            }
            Token::Top => {
                self.advance();
                let n = self.expect_integer()?;
                if n < 0 {
                    return Err(LqlError::Compile {
                        message: "top count must be non-negative".to_string(),
                        span: None,
                    });
                }
                let count = n as usize;
                let order = if self.check(&Token::Asc) {
                    self.advance();
                    Order::Asc
                } else if self.check(&Token::Desc) {
                    self.advance();
                    Order::Desc
                } else {
                    Order::Desc
                };
                self.expect_keyword(&Token::By)?;
                let by = self.parse_expr_list()?;
                Ok(Statement::Top { count, by, order })
            }
            Token::Timeseries => {
                self.advance();
                let interval = self.expect_duration()?;
                Ok(Statement::Timeseries { interval })
            }
            _ => Err(self.unexpected("statement keyword (where, summarize, sort, limit, project, extend, top, timeseries)")),
        }
    }

    fn parse_agg_list(&mut self) -> Result<Vec<AggExpr>, LqlError> {
        let mut aggs = Vec::new();
        aggs.push(self.parse_agg()?);
        while self.check(&Token::Comma) {
            self.advance();
            aggs.push(self.parse_agg()?);
        }
        Ok(aggs)
    }

    fn parse_agg(&mut self) -> Result<AggExpr, LqlError> {
        // Support both `count() as cnt` and `cnt = count()` syntax
        let leading_alias = if let Token::Ident(name) = self.current().clone() {
            if self.pos + 1 < self.tokens.len() && self.tokens[self.pos + 1] == Token::Eq {
                self.advance(); // consume ident
                self.advance(); // consume =
                Some(name)
            } else {
                None
            }
        } else {
            None
        };

        let (function, arg) = match self.current().clone() {
            Token::Count => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = if !self.check(&Token::RParen) {
                    Some(self.parse_expr()?)
                } else {
                    None
                };
                self.expect_token(&Token::RParen)?;
                (AggFunction::Count, arg)
            }
            Token::Sum => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::Sum, Some(arg))
            }
            Token::Avg => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::Avg, Some(arg))
            }
            Token::Min => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::Min, Some(arg))
            }
            Token::Max => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::Max, Some(arg))
            }
            Token::P50 => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::P50, Some(arg))
            }
            Token::P95 => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::P95, Some(arg))
            }
            Token::P99 => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::P99, Some(arg))
            }
            Token::DCount => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::DCount, Some(arg))
            }
            Token::First => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::First, Some(arg))
            }
            Token::Last => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let arg = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                (AggFunction::Last, Some(arg))
            }
            _ => return Err(self.unexpected("aggregation function")),
        };

        // Optional trailing alias: "as name"
        let trailing_alias = if self.check(&Token::As) {
            self.advance();
            Some(self.expect_ident()?)
        } else {
            None
        };

        let alias = leading_alias.or(trailing_alias);

        Ok(AggExpr {
            function,
            arg,
            alias,
        })
    }

    fn parse_expr_list(&mut self) -> Result<Vec<Expr>, LqlError> {
        let mut exprs = Vec::new();
        exprs.push(self.parse_expr()?);
        while self.check(&Token::Comma) {
            self.advance();
            exprs.push(self.parse_expr()?);
        }
        Ok(exprs)
    }

    // Expression parsing with precedence climbing
    fn parse_expr(&mut self) -> Result<Expr, LqlError> {
        self.parse_or_expr()
    }

    fn parse_or_expr(&mut self) -> Result<Expr, LqlError> {
        let mut left = self.parse_and_expr()?;
        while self.check(&Token::Or) {
            self.advance();
            let right = self.parse_and_expr()?;
            left = Expr::BinaryOp {
                left: Box::new(left),
                op: BinOp::Or,
                right: Box::new(right),
            };
        }
        Ok(left)
    }

    fn parse_and_expr(&mut self) -> Result<Expr, LqlError> {
        let mut left = self.parse_not_expr()?;
        while self.check(&Token::And) {
            self.advance();
            let right = self.parse_not_expr()?;
            left = Expr::BinaryOp {
                left: Box::new(left),
                op: BinOp::And,
                right: Box::new(right),
            };
        }
        Ok(left)
    }

    fn parse_not_expr(&mut self) -> Result<Expr, LqlError> {
        if self.check(&Token::Not) {
            self.advance();
            let expr = self.parse_comparison()?;
            Ok(Expr::UnaryOp {
                op: UnaryOp::Not,
                expr: Box::new(expr),
            })
        } else {
            self.parse_comparison()
        }
    }

    fn parse_comparison(&mut self) -> Result<Expr, LqlError> {
        let mut left = self.parse_additive()?;

        let op = match self.current() {
            Token::Eq => Some(BinOp::Eq),
            Token::Neq => Some(BinOp::Neq),
            Token::Gt => Some(BinOp::Gt),
            Token::Lt => Some(BinOp::Lt),
            Token::Gte => Some(BinOp::Gte),
            Token::Lte => Some(BinOp::Lte),
            Token::Like => Some(BinOp::Like),
            Token::NotLike => Some(BinOp::NotLike),
            Token::Contains => Some(BinOp::Contains),
            Token::Has => Some(BinOp::Has),
            Token::StartsWith => Some(BinOp::StartsWith),
            Token::EndsWith => Some(BinOp::EndsWith),
            Token::In => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let values = self.parse_expr_list()?;
                self.expect_token(&Token::RParen)?;
                return Ok(Expr::InList {
                    expr: Box::new(left),
                    values,
                });
            }
            _ => None,
        };

        if let Some(op) = op {
            self.advance();
            let right = self.parse_additive()?;
            left = Expr::BinaryOp {
                left: Box::new(left),
                op,
                right: Box::new(right),
            };
        }

        Ok(left)
    }

    fn parse_additive(&mut self) -> Result<Expr, LqlError> {
        let mut left = self.parse_multiplicative()?;
        loop {
            let op = match self.current() {
                Token::Plus => BinOp::Add,
                Token::Minus => BinOp::Sub,
                _ => break,
            };
            self.advance();
            let right = self.parse_multiplicative()?;
            left = Expr::BinaryOp {
                left: Box::new(left),
                op,
                right: Box::new(right),
            };
        }
        Ok(left)
    }

    fn parse_multiplicative(&mut self) -> Result<Expr, LqlError> {
        let mut left = self.parse_unary()?;
        loop {
            let op = match self.current() {
                Token::Star => BinOp::Mul,
                Token::Slash => BinOp::Div,
                Token::Percent => BinOp::Mod,
                _ => break,
            };
            self.advance();
            let right = self.parse_unary()?;
            left = Expr::BinaryOp {
                left: Box::new(left),
                op,
                right: Box::new(right),
            };
        }
        Ok(left)
    }

    fn parse_unary(&mut self) -> Result<Expr, LqlError> {
        match self.current() {
            Token::Minus => {
                self.advance();
                let expr = self.parse_primary()?;
                Ok(Expr::UnaryOp {
                    op: UnaryOp::Neg,
                    expr: Box::new(expr),
                })
            }
            _ => self.parse_primary(),
        }
    }

    fn parse_primary(&mut self) -> Result<Expr, LqlError> {
        if self.pos >= self.tokens.len() {
            return Err(LqlError::UnexpectedEof);
        }
        match self.current().clone() {
            Token::StringLit(s) => {
                self.advance();
                Ok(Expr::Literal(Literal::String(s)))
            }
            Token::IntegerLit(n) => {
                self.advance();
                Ok(Expr::Literal(Literal::Integer(n)))
            }
            Token::FloatLit(n) => {
                self.advance();
                Ok(Expr::Literal(Literal::Float(n)))
            }
            Token::BoolLit(b) => {
                self.advance();
                Ok(Expr::Literal(Literal::Bool(b)))
            }
            Token::NullLit => {
                self.advance();
                Ok(Expr::Literal(Literal::Null))
            }
            Token::DurationLit(d) => {
                self.advance();
                Ok(Expr::Literal(Literal::Duration(d)))
            }
            Token::Star => {
                self.advance();
                Ok(Expr::Wildcard)
            }
            Token::LParen => {
                self.advance();
                let expr = self.parse_expr()?;
                self.expect_token(&Token::RParen)?;
                Ok(expr)
            }
            Token::Ident(name) => {
                let name = name.clone();
                self.advance();

                // Handle dotted column names like user.id
                let mut full_name = name;
                while self.check(&Token::Dot) {
                    self.advance();
                    let part = self.expect_ident()?;
                    full_name.push('.');
                    full_name.push_str(&part);
                }

                // Check if it's a function call
                if self.check(&Token::LParen) {
                    self.advance();
                    let mut args = Vec::new();
                    if !self.check(&Token::RParen) {
                        args = self.parse_expr_list()?;
                    }
                    self.expect_token(&Token::RParen)?;
                    Ok(Expr::Function {
                        name: full_name,
                        args,
                    })
                } else {
                    Ok(Expr::Column(full_name))
                }
            }
            Token::Ago => {
                self.advance();
                self.expect_token(&Token::LParen)?;
                let dur = self.expect_duration()?;
                self.expect_token(&Token::RParen)?;
                Ok(Expr::Function {
                    name: "ago".to_string(),
                    args: vec![Expr::Literal(Literal::Duration(dur))],
                })
            }
            // Allow aggregation keywords to be used as column names (e.g., `sort count desc`)
            Token::Count | Token::Sum | Token::Avg | Token::Min | Token::Max => {
                let name = format!("{:?}", self.current()).to_lowercase();
                self.advance();
                // If followed by `(`, treat as function call
                if self.check(&Token::LParen) {
                    self.advance();
                    let mut args = Vec::new();
                    if !self.check(&Token::RParen) {
                        args = self.parse_expr_list()?;
                    }
                    self.expect_token(&Token::RParen)?;
                    Ok(Expr::Function { name, args })
                } else {
                    Ok(Expr::Column(name))
                }
            }
            _ => Err(self.unexpected("expression")),
        }
    }

    // Helpers
    fn current(&self) -> &Token {
        self.tokens.get(self.pos).unwrap_or(&Token::NullLit)
    }

    fn advance(&mut self) {
        if self.pos < self.tokens.len() {
            self.pos += 1;
        }
    }

    fn check(&self, token: &Token) -> bool {
        self.current() == token
    }

    fn expect_token(&mut self, expected: &Token) -> Result<(), LqlError> {
        if self.current() == expected {
            self.advance();
            Ok(())
        } else {
            Err(self.unexpected(&format!("{}", expected)))
        }
    }

    fn expect_keyword(&mut self, keyword: &Token) -> Result<(), LqlError> {
        self.expect_token(keyword)
    }

    fn expect_ident(&mut self) -> Result<String, LqlError> {
        match self.current().clone() {
            Token::Ident(name) => {
                self.advance();
                Ok(name)
            }
            _ => Err(self.unexpected("identifier")),
        }
    }

    fn expect_integer(&mut self) -> Result<i64, LqlError> {
        match self.current() {
            Token::IntegerLit(n) => {
                let n = *n;
                self.advance();
                Ok(n)
            }
            _ => Err(self.unexpected("integer")),
        }
    }

    fn expect_duration(&mut self) -> Result<Duration, LqlError> {
        match self.current().clone() {
            Token::DurationLit(d) => {
                self.advance();
                Ok(d)
            }
            _ => Err(self.unexpected("duration (e.g. 1h, 30m, 7d)")),
        }
    }

    fn unexpected(&self, expected: &str) -> LqlError {
        if self.pos >= self.tokens.len() {
            return LqlError::UnexpectedEof;
        }
        let token = self.current().clone();
        let pos = self.pos;
        LqlError::UnexpectedToken {
            token: format!("{}", token),
            expected: expected.to_string(),
            pos,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::lexer::Lexer;

    fn parse(input: &str) -> Pipeline {
        let tokens = Lexer::new(input).tokenize().unwrap();
        Parser::new(tokens).parse().unwrap()
    }

    #[test]
    fn simple_from_where_limit() {
        let p = parse(r#"from events | where level = "error" | limit 10"#);
        assert_eq!(p.statements.len(), 3);
        assert!(matches!(&p.statements[0], Statement::From(Source::Events)));
        assert!(matches!(&p.statements[1], Statement::Where(_)));
        assert!(matches!(&p.statements[2], Statement::Limit(10)));
    }

    #[test]
    fn summarize_with_by() {
        let p = parse(r#"from events | summarize count(), avg(duration_ms) by service"#);
        assert_eq!(p.statements.len(), 2);
        if let Statement::Summarize { aggregations, by } = &p.statements[1] {
            assert_eq!(aggregations.len(), 2);
            assert_eq!(by.len(), 1);
        } else {
            panic!("expected Summarize");
        }
    }

    #[test]
    fn where_with_and() {
        let p = parse(r#"from events | where level = "error" and service = "checkout""#);
        if let Statement::Where(Expr::BinaryOp { op, .. }) = &p.statements[1] {
            assert_eq!(*op, BinOp::And);
        } else {
            panic!("expected BinaryOp AND");
        }
    }

    #[test]
    fn sort_desc() {
        let p = parse("from events | sort duration_ms desc | limit 5");
        if let Statement::Sort { order, .. } = &p.statements[1] {
            assert_eq!(*order, Order::Desc);
        } else {
            panic!("expected Sort");
        }
    }

    #[test]
    fn project_extend() {
        let p =
            parse("from events | project service, event | extend is_error = (level = \"error\")");
        assert_eq!(p.statements.len(), 3);
        assert!(matches!(&p.statements[1], Statement::Project(_)));
        assert!(matches!(&p.statements[2], Statement::Extend { .. }));
    }

    #[test]
    fn timeseries() {
        let p = parse("from events | timeseries 5m");
        assert!(matches!(&p.statements[1], Statement::Timeseries { .. }));
    }

    #[test]
    fn nested_column_in_where() {
        let p = parse(r#"from events | where user.id = "u123""#);
        if let Statement::Where(Expr::BinaryOp { .. }) = &p.statements[1] {
            // user.id is parsed as Column("user") Dot Column("id") — need dotted column support
            // For now, just verify it parses
        }
    }
}
