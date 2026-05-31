use serde::{Deserialize, Serialize};

/// A complete LQL query represented as a pipeline of statements.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Pipeline {
    pub statements: Vec<Statement>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Statement {
    From(Source),
    Where(Expr),
    Summarize {
        aggregations: Vec<AggExpr>,
        by: Vec<Expr>,
    },
    Sort {
        field: Expr,
        order: Order,
    },
    Limit(usize),
    Project(Vec<Expr>),
    Extend {
        name: String,
        expr: Expr,
    },
    Top {
        count: usize,
        by: Vec<Expr>,
        order: Order,
    },
    Timeseries {
        interval: Duration,
    },
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Source {
    Events,
    Traces,
    Incidents,
    Table(String),
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Expr {
    Column(String),
    Literal(Literal),
    BinaryOp {
        left: Box<Expr>,
        op: BinOp,
        right: Box<Expr>,
    },
    UnaryOp {
        op: UnaryOp,
        expr: Box<Expr>,
    },
    Function {
        name: String,
        args: Vec<Expr>,
    },
    InList {
        expr: Box<Expr>,
        values: Vec<Expr>,
    },
    Wildcard,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum Literal {
    String(String),
    Integer(i64),
    Float(f64),
    Bool(bool),
    Null,
    Duration(Duration),
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Duration {
    pub value: u64,
    pub unit: DurationUnit,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum DurationUnit {
    Milliseconds,
    Seconds,
    Minutes,
    Hours,
    Days,
    Weeks,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BinOp {
    // Comparison
    Eq,
    Neq,
    Gt,
    Lt,
    Gte,
    Lte,
    // String
    Like,
    NotLike,
    Contains,
    Has,
    StartsWith,
    EndsWith,
    // Logical
    And,
    Or,
    // Arithmetic
    Add,
    Sub,
    Mul,
    Div,
    Mod,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum UnaryOp {
    Not,
    Neg,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Order {
    Asc,
    Desc,
}

/// An aggregation expression used in `summarize`.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AggExpr {
    pub function: AggFunction,
    pub arg: Option<Expr>,
    pub alias: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum AggFunction {
    Count,
    Sum,
    Avg,
    Min,
    Max,
    P50,
    P95,
    P99,
    Percentile(f64),
    DCount,
    First,
    Last,
}

impl Duration {
    pub fn to_millis(&self) -> u64 {
        match self.unit {
            DurationUnit::Milliseconds => self.value,
            DurationUnit::Seconds => self.value.saturating_mul(1000),
            DurationUnit::Minutes => self.value.saturating_mul(60).saturating_mul(1000),
            DurationUnit::Hours => self.value.saturating_mul(3600).saturating_mul(1000),
            DurationUnit::Days => self.value.saturating_mul(86400).saturating_mul(1000),
            DurationUnit::Weeks => self.value.saturating_mul(604800).saturating_mul(1000),
        }
    }
}

impl Source {
    /// Returns the default table name for this source.
    pub fn table_name(&self) -> &str {
        match self {
            Source::Events => "events",
            Source::Traces => "traces",
            Source::Incidents => "incidents",
            Source::Table(name) => name,
        }
    }
}

impl std::fmt::Display for Order {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Order::Asc => write!(f, "ASC"),
            Order::Desc => write!(f, "DESC"),
        }
    }
}

impl std::fmt::Display for BinOp {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            BinOp::Eq => write!(f, "="),
            BinOp::Neq => write!(f, "!="),
            BinOp::Gt => write!(f, ">"),
            BinOp::Lt => write!(f, "<"),
            BinOp::Gte => write!(f, ">="),
            BinOp::Lte => write!(f, "<="),
            BinOp::Like => write!(f, "like"),
            BinOp::NotLike => write!(f, "not like"),
            BinOp::Contains => write!(f, "contains"),
            BinOp::Has => write!(f, "has"),
            BinOp::StartsWith => write!(f, "startswith"),
            BinOp::EndsWith => write!(f, "endswith"),
            BinOp::And => write!(f, "AND"),
            BinOp::Or => write!(f, "OR"),
            BinOp::Add => write!(f, "+"),
            BinOp::Sub => write!(f, "-"),
            BinOp::Mul => write!(f, "*"),
            BinOp::Div => write!(f, "/"),
            BinOp::Mod => write!(f, "%"),
        }
    }
}
