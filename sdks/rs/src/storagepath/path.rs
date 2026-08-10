use std::path::PathBuf;

pub fn default_storage_dir() -> PathBuf {
    std::env::temp_dir().join("loza")
}

pub fn spool_path(name: &str) -> PathBuf {
    default_storage_dir().join(format!("{name}.spool"))
}
