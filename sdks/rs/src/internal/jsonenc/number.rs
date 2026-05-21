pub fn finite_f64(value: f64) -> Option<f64> {
    value.is_finite().then_some(value)
}

pub fn clamp_u64(value: u128) -> u64 {
    value.min(u64::MAX as u128) as u64
}
