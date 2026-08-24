use std::collections::VecDeque;
use std::fs::{self};
use std::mem;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};

pub trait WritableSink {
    fn write(&self, encoded: &str) -> Result<(), String>;
}

pub trait BatchWritableSink {
    fn write_batch(&self, encoded_events: &[String]) -> Result<(), String>;
}

#[derive(Clone, Debug, Default)]
pub struct DeliveryStats {
    pub enqueued: u64,
    pub emitted: u64,
    pub dropped: u64,
    pub failed: u64,
    pub retried: u64,
    pub batches: u64,
    pub last_error: String,
}

impl DeliveryStats {
    pub fn snapshot(&self) -> Self {
        self.clone()
    }
}

pub struct ByteBatcher {
    max_batch_bytes: usize,
    max_events: usize,
    current_bytes: usize,
    events: Vec<String>,
}

impl ByteBatcher {
    pub fn new(max_batch_bytes: usize) -> Self {
        Self::with_limits(max_batch_bytes, 1024)
    }

    pub fn with_limits(max_batch_bytes: usize, max_events: usize) -> Self {
        Self {
            max_batch_bytes: max_batch_bytes.max(1),
            max_events: max_events.max(1),
            current_bytes: 0,
            events: Vec::new(),
        }
    }

    pub fn push(&mut self, event: String) -> Option<Vec<String>> {
        let size = event.len();
        let should_flush = self.current_bytes > 0
            && (self.current_bytes + size > self.max_batch_bytes
                || self.events.len() >= self.max_events);
        if should_flush {
            let flushed = self.drain();
            self.events.push(event);
            self.current_bytes = size;
            Some(flushed)
        } else {
            self.current_bytes += size;
            self.events.push(event);
            None
        }
    }

    pub fn drain(&mut self) -> Vec<String> {
        let out = mem::take(&mut self.events);
        self.current_bytes = 0;
        out
    }

    pub fn len(&self) -> usize {
        self.events.len()
    }

    pub fn is_empty(&self) -> bool {
        self.events.is_empty()
    }
}

pub struct MemoryOfflineBuffer {
    queue: Arc<Mutex<VecDeque<String>>>,
    max_size: usize,
}

impl MemoryOfflineBuffer {
    pub fn new(max_size: usize) -> Self {
        Self {
            queue: Arc::new(Mutex::new(VecDeque::new())),
            max_size,
        }
    }

    pub fn enqueue(&self, event: String) -> bool {
        let mut q = self.queue.lock().unwrap();
        if q.len() >= self.max_size {
            q.pop_front();
            q.push_back(event);
            false
        } else {
            q.push_back(event);
            true
        }
    }

    pub fn drain(&self) -> Vec<String> {
        let mut q = self.queue.lock().unwrap();
        q.drain(..).collect()
    }

    pub fn len(&self) -> usize {
        self.queue.lock().unwrap().len()
    }

    pub fn is_empty(&self) -> bool {
        self.queue.lock().unwrap().is_empty()
    }
}

pub struct DiskOfflineBuffer {
    dir: PathBuf,
    max_files: usize,
}

impl DiskOfflineBuffer {
    pub fn new(dir: impl AsRef<Path>, max_files: usize) -> Self {
        let dir = dir.as_ref().to_path_buf();
        fs::create_dir_all(&dir).ok();
        Self { dir, max_files }
    }

    pub fn enqueue(&self, event: &str) -> std::io::Result<()> {
        let filename = format!("{}.json", crate::internal::clock::unix_millis());
        let path = self.dir.join(filename);
        fs::write(path, event)?;
        self.evict_old_files();
        Ok(())
    }

    pub fn drain(&self) -> std::io::Result<Vec<String>> {
        let mut events = Vec::new();
        let mut entries: Vec<_> = fs::read_dir(&self.dir)?
            .filter_map(|e| e.ok())
            .filter(|e| {
                e.path()
                    .extension()
                    .map(|ext| ext == "json")
                    .unwrap_or(false)
            })
            .collect();
        entries.sort_by_key(|e| e.file_name());
        for entry in entries {
            let path = entry.path();
            match fs::read_to_string(&path) {
                Ok(content) => {
                    events.push(content);
                    fs::remove_file(path).ok();
                }
                Err(_) => continue,
            }
        }
        Ok(events)
    }

    fn evict_old_files(&self) {
        let mut entries: Vec<_> = match fs::read_dir(&self.dir) {
            Ok(rd) => rd.filter_map(|e| e.ok()).collect(),
            Err(_) => return,
        };
        if entries.len() <= self.max_files {
            return;
        }
        entries.sort_by_key(|e| e.file_name());
        let to_remove = entries.len() - self.max_files;
        for entry in entries.into_iter().take(to_remove) {
            fs::remove_file(entry.path()).ok();
        }
    }
}
