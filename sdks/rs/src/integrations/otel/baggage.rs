use std::collections::BTreeMap;

pub fn baggage_attrs(
    items: impl IntoIterator<Item = (String, String)>,
) -> BTreeMap<String, String> {
    items.into_iter().collect()
}
