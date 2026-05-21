#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum DuplicatePolicy {
    CanonicalWins,
    UserWins,
    FirstWins,
    LastWins,
    KeepBoth,
    ErrorOnDuplicate,
}

impl DuplicatePolicy {
    pub fn parse(value: &str) -> Self {
        match value {
            "canonical_wins" => Self::CanonicalWins,
            "user_wins" => Self::UserWins,
            "first_wins" => Self::FirstWins,
            "keep_both" => Self::KeepBoth,
            "error_on_duplicate" => Self::ErrorOnDuplicate,
            _ => Self::LastWins,
        }
    }
}
