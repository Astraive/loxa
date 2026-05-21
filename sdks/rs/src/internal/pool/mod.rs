#[derive(Default)]
pub struct StringPool {
    strings: Vec<String>,
}

impl StringPool {
    pub fn take(&mut self) -> String {
        self.strings.pop().unwrap_or_default()
    }

    pub fn put(&mut self, mut value: String) {
        value.clear();
        self.strings.push(value);
    }
}
