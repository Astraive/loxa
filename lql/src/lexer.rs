use crate::ast::{Duration, DurationUnit};
use crate::error::LqlError;

#[derive(Debug, Clone, PartialEq)]
pub enum Token {
    // Literals
    StringLit(String),
    IntegerLit(i64),
    FloatLit(f64),
    BoolLit(bool),
    NullLit,
    DurationLit(Duration),

    // Identifiers
    Ident(String),

    // Keywords
    From,
    Where,
    Summarize,
    By,
    Sort,
    Limit,
    Project,
    Extend,
    Top,
    Timeseries,
    As,
    Asc,
    Desc,
    And,
    Or,
    Not,
    In,
    Like,
    NotLike,
    Contains,
    Has,
    StartsWith,
    EndsWith,
    Ago,

    // Aggregation functions
    Count,
    Sum,
    Avg,
    Min,
    Max,
    P50,
    P95,
    P99,
    DCount,
    First,
    Last,

    // Operators
    Eq,      // =
    Neq,     // !=
    Gt,      // >
    Lt,      // <
    Gte,     // >=
    Lte,     // <=
    Plus,    // +
    Minus,   // -
    Star,    // *
    Slash,   // /
    Percent, // %
    LParen,  // (
    RParen,  // )
    Comma,   // ,
    Dot,     // .

    // Pipe
    Pipe, // |
}

impl std::fmt::Display for Token {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Token::StringLit(s) => write!(f, "\"{}\"", s),
            Token::IntegerLit(n) => write!(f, "{}", n),
            Token::FloatLit(n) => write!(f, "{}", n),
            Token::BoolLit(b) => write!(f, "{}", b),
            Token::NullLit => write!(f, "null"),
            Token::DurationLit(d) => write!(f, "{}{:?}", d.value, d.unit),
            Token::Ident(s) => write!(f, "{}", s),
            _ => write!(f, "{:?}", self),
        }
    }
}

pub struct Lexer {
    input: Vec<char>,
    pos: usize,
}

impl Lexer {
    pub fn new(input: &str) -> Self {
        Self {
            input: input.chars().collect(),
            pos: 0,
        }
    }

    pub fn tokenize(&mut self) -> Result<Vec<Token>, LqlError> {
        let mut tokens = Vec::new();
        while self.pos < self.input.len() {
            self.skip_whitespace();
            if self.pos >= self.input.len() {
                break;
            }
            let ch = self.input[self.pos];
            match ch {
                '|' => {
                    if self.peek_ahead(1) == Some('|') {
                        tokens.push(Token::Or);
                        self.pos += 2;
                    } else {
                        tokens.push(Token::Pipe);
                        self.pos += 1;
                    }
                }
                '(' => {
                    tokens.push(Token::LParen);
                    self.pos += 1;
                }
                ')' => {
                    tokens.push(Token::RParen);
                    self.pos += 1;
                }
                ',' => {
                    tokens.push(Token::Comma);
                    self.pos += 1;
                }
                '.' => {
                    tokens.push(Token::Dot);
                    self.pos += 1;
                }
                '+' => {
                    tokens.push(Token::Plus);
                    self.pos += 1;
                }
                '-' => {
                    // Could be minus or start of negative number
                    tokens.push(Token::Minus);
                    self.pos += 1;
                }
                '*' => {
                    tokens.push(Token::Star);
                    self.pos += 1;
                }
                '/' => {
                    tokens.push(Token::Slash);
                    self.pos += 1;
                }
                '%' => {
                    tokens.push(Token::Percent);
                    self.pos += 1;
                }
                '=' => {
                    if self.peek_ahead(1) == Some('~') {
                        tokens.push(Token::Like);
                        self.pos += 2;
                    } else {
                        tokens.push(Token::Eq);
                        self.pos += 1;
                    }
                }
                '!' => {
                    if self.peek_ahead(1) == Some('=') {
                        tokens.push(Token::Neq);
                        self.pos += 2;
                    } else if self.peek_ahead(1) == Some('~') {
                        // !~ not-like operator
                        tokens.push(Token::NotLike);
                        self.pos += 2;
                    } else {
                        return Err(LqlError::UnexpectedChar {
                            char: ch,
                            pos: self.pos,
                        });
                    }
                }
                '>' => {
                    if self.peek_ahead(1) == Some('=') {
                        tokens.push(Token::Gte);
                        self.pos += 2;
                    } else {
                        tokens.push(Token::Gt);
                        self.pos += 1;
                    }
                }
                '<' => {
                    if self.peek_ahead(1) == Some('=') {
                        tokens.push(Token::Lte);
                        self.pos += 2;
                    } else {
                        tokens.push(Token::Lt);
                        self.pos += 1;
                    }
                }
                '"' | '\'' => {
                    tokens.push(Token::StringLit(self.read_string()?));
                }
                '&' if self.peek_ahead(1) == Some('&') => {
                    tokens.push(Token::And);
                    self.pos += 2;
                }
                '#' => {
                    // Line comment — skip to end
                    while self.pos < self.input.len() && self.input[self.pos] != '\n' {
                        self.pos += 1;
                    }
                }
                _ if ch.is_ascii_digit() => {
                    tokens.push(self.read_number()?);
                }
                _ if ch.is_ascii_alphabetic() || ch == '_' => {
                    tokens.push(self.read_ident_or_keyword()?);
                }
                _ => {
                    return Err(LqlError::UnexpectedChar {
                        char: ch,
                        pos: self.pos,
                    });
                }
            }
        }
        Ok(tokens)
    }

    fn skip_whitespace(&mut self) {
        while self.pos < self.input.len() && self.input[self.pos].is_ascii_whitespace() {
            self.pos += 1;
        }
    }

    fn peek_ahead(&self, offset: usize) -> Option<char> {
        self.input.get(self.pos + offset).copied()
    }

    fn read_string(&mut self) -> Result<String, LqlError> {
        let quote = self.input[self.pos];
        self.pos += 1; // skip opening quote
        let _start = self.pos;
        let mut result = String::new();
        while self.pos < self.input.len() {
            let ch = self.input[self.pos];
            if ch == quote {
                self.pos += 1; // skip closing quote
                return Ok(result);
            }
            if ch == '\\' && self.pos + 1 < self.input.len() {
                self.pos += 1;
                match self.input[self.pos] {
                    'n' => result.push('\n'),
                    't' => result.push('\t'),
                    '\\' => result.push('\\'),
                    c if c == quote => result.push(c),
                    c => {
                        result.push('\\');
                        result.push(c);
                    }
                }
            } else {
                result.push(ch);
            }
            self.pos += 1;
        }
        Err(LqlError::UnexpectedEof)
    }

    fn read_number(&mut self) -> Result<Token, LqlError> {
        let start = self.pos;
        while self.pos < self.input.len() && self.input[self.pos].is_ascii_digit() {
            self.pos += 1;
        }
        // Check for duration suffix (1h, 30m, 7d, etc.)
        if self.pos < self.input.len() && self.input[self.pos].is_ascii_alphabetic() {
            let unit_start = self.pos;
            while self.pos < self.input.len() && self.input[self.pos].is_ascii_alphabetic() {
                self.pos += 1;
            }
            let num_text: String = self.input[start..unit_start].iter().collect();
            let unit: String = self.input[unit_start..self.pos].iter().collect();
            if let Ok(value) = num_text.parse::<u64>() {
                let duration_unit = match unit.to_lowercase().as_str() {
                    "ms" => DurationUnit::Milliseconds,
                    "s" | "sec" | "secs" | "second" | "seconds" => DurationUnit::Seconds,
                    "m" | "min" | "mins" | "minute" | "minutes" => DurationUnit::Minutes,
                    "h" | "hr" | "hrs" | "hour" | "hours" => DurationUnit::Hours,
                    "d" | "day" | "days" => DurationUnit::Days,
                    "w" | "week" | "weeks" => DurationUnit::Weeks,
                    _ => {
                        // Not a valid duration — treat as number + identifier
                        self.pos = unit_start;
                        let text: String = self.input[start..self.pos].iter().collect();
                        return text.parse::<i64>().map(Token::IntegerLit).map_err(|_| {
                            LqlError::UnexpectedChar {
                                char: '0',
                                pos: start,
                            }
                        });
                    }
                };
                return Ok(Token::DurationLit(Duration {
                    value,
                    unit: duration_unit,
                }));
            }
        }
        // Check for float
        if self.pos < self.input.len() && self.input[self.pos] == '.' {
            self.pos += 1;
            while self.pos < self.input.len() && self.input[self.pos].is_ascii_digit() {
                self.pos += 1;
            }
            let text: String = self.input[start..self.pos].iter().collect();
            return text.parse::<f64>().map(Token::FloatLit).map_err(|_| {
                LqlError::UnexpectedChar {
                    char: '.',
                    pos: start,
                }
            });
        }
        let text: String = self.input[start..self.pos].iter().collect();
        text.parse::<i64>()
            .map(Token::IntegerLit)
            .map_err(|_| LqlError::UnexpectedChar {
                char: '0',
                pos: start,
            })
    }

    fn read_ident_or_keyword(&mut self) -> Result<Token, LqlError> {
        let start = self.pos;
        while self.pos < self.input.len()
            && (self.input[self.pos].is_ascii_alphanumeric() || self.input[self.pos] == '_')
        {
            self.pos += 1;
        }
        let word: String = self.input[start..self.pos].iter().collect();
        let lower = word.to_lowercase();

        // Check for duration literals like 1h, 30m, 7d
        if self.pos < self.input.len() && self.input[self.pos].is_ascii_alphabetic() {
            let unit_start = self.pos;
            while self.pos < self.input.len() && self.input[self.pos].is_ascii_alphabetic() {
                self.pos += 1;
            }
            let unit: String = self.input[unit_start..self.pos].iter().collect();
            if let Ok(value) = word.parse::<u64>() {
                let duration_unit = match unit.to_lowercase().as_str() {
                    "ms" => DurationUnit::Milliseconds,
                    "s" | "sec" | "secs" | "second" | "seconds" => DurationUnit::Seconds,
                    "m" | "min" | "mins" | "minute" | "minutes" => DurationUnit::Minutes,
                    "h" | "hr" | "hrs" | "hour" | "hours" => DurationUnit::Hours,
                    "d" | "day" | "days" => DurationUnit::Days,
                    "w" | "week" | "weeks" => DurationUnit::Weeks,
                    _ => {
                        // Not a duration, put unit back as ident
                        self.pos = unit_start;
                        return Ok(self.keyword_or_ident(word));
                    }
                };
                return Ok(Token::DurationLit(Duration {
                    value,
                    unit: duration_unit,
                }));
            }
            // Not a number followed by unit, reset
            self.pos = unit_start;
        }

        match lower.as_str() {
            "from" => Ok(Token::From),
            "where" => Ok(Token::Where),
            "summarize" => Ok(Token::Summarize),
            "by" => Ok(Token::By),
            "sort" | "order" => Ok(Token::Sort),
            "limit" | "take" => Ok(Token::Limit),
            "project" | "select" => Ok(Token::Project),
            "extend" => Ok(Token::Extend),
            "top" => Ok(Token::Top),
            "timeseries" | "time_series" => Ok(Token::Timeseries),
            "as" => Ok(Token::As),
            "asc" | "ascending" => Ok(Token::Asc),
            "desc" | "descending" => Ok(Token::Desc),
            "and" => Ok(Token::And),
            "or" => Ok(Token::Or),
            "not" => Ok(Token::Not),
            "in" => Ok(Token::In),
            "like" => Ok(Token::Like),
            "contains" | "contain" => Ok(Token::Contains),
            "has" => Ok(Token::Has),
            "startswith" | "starts_with" => Ok(Token::StartsWith),
            "endswith" | "ends_with" => Ok(Token::EndsWith),
            "ago" => Ok(Token::Ago),
            "count" => Ok(Token::Count),
            "sum" => Ok(Token::Sum),
            "avg" | "average" => Ok(Token::Avg),
            "min" => Ok(Token::Min),
            "max" => Ok(Token::Max),
            "p50" | "median" => Ok(Token::P50),
            "p95" => Ok(Token::P95),
            "p99" => Ok(Token::P99),
            "dcount" | "dcountif" => Ok(Token::DCount),
            "first" => Ok(Token::First),
            "last" => Ok(Token::Last),
            "true" => Ok(Token::BoolLit(true)),
            "false" => Ok(Token::BoolLit(false)),
            "null" | "none" => Ok(Token::NullLit),
            _ => Ok(Token::Ident(word)),
        }
    }

    fn keyword_or_ident(&self, word: String) -> Token {
        // This is a fallback when duration parsing fails
        Token::Ident(word)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn tokenize(input: &str) -> Vec<Token> {
        Lexer::new(input).tokenize().unwrap()
    }

    #[test]
    fn basic_pipeline() {
        let tokens = tokenize(r#"from events | where level = "error" | limit 10"#);
        assert_eq!(tokens[0], Token::From);
        assert_eq!(tokens[1], Token::Ident("events".into()));
        assert_eq!(tokens[2], Token::Pipe);
        assert_eq!(tokens[3], Token::Where);
        assert_eq!(tokens[4], Token::Ident("level".into()));
        assert_eq!(tokens[5], Token::Eq);
        assert_eq!(tokens[6], Token::StringLit("error".into()));
        assert_eq!(tokens[7], Token::Pipe);
        assert_eq!(tokens[8], Token::Limit);
        assert_eq!(tokens[9], Token::IntegerLit(10));
    }

    #[test]
    fn duration_literal() {
        let tokens = tokenize("1h");
        assert_eq!(
            tokens[0],
            Token::DurationLit(Duration {
                value: 1,
                unit: DurationUnit::Hours
            })
        );
    }

    #[test]
    fn comparison_operators() {
        let tokens = tokenize("a >= 1 and b != 2 or c < 3");
        assert_eq!(tokens[1], Token::Gte);
        assert_eq!(tokens[3], Token::And);
        assert_eq!(tokens[5], Token::Neq);
        assert_eq!(tokens[7], Token::Or);
        assert_eq!(tokens[9], Token::Lt);
    }

    #[test]
    fn string_with_escape() {
        let tokens = tokenize(r#""hello\nworld""#);
        assert_eq!(tokens[0], Token::StringLit("hello\nworld".into()));
    }

    #[test]
    fn nested_field() {
        let tokens = tokenize("user.name");
        assert_eq!(tokens[0], Token::Ident("user".into()));
        assert_eq!(tokens[1], Token::Dot);
        assert_eq!(tokens[2], Token::Ident("name".into()));
    }
}
