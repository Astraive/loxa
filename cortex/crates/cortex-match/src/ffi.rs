use std::ffi::{CStr, CString};
use std::os::raw::c_char;

use crate::{Matcher, Signature};

fn cstr_to_string(ptr: *const c_char) -> Option<String> {
    if ptr.is_null() {
        return None;
    }
    unsafe { CStr::from_ptr(ptr) }.to_str().ok().map(str::to_owned)
}

/// Score a query signature against a candidate signature.
/// Both arguments are JSON-serialized Signature objects.
/// Returns the similarity score as f64.
#[no_mangle]
pub extern "C" fn score_ffi(
    query_json: *const c_char,
    candidate_json: *const c_char,
) -> f64 {
    let query_str = match cstr_to_string(query_json) {
        Some(s) => s,
        None => return 0.0,
    };
    let candidate_str = match cstr_to_string(candidate_json) {
        Some(s) => s,
        None => return 0.0,
    };

    let query: Signature = match serde_json::from_str(&query_str) {
        Ok(s) => s,
        Err(_) => return 0.0,
    };
    let candidate: Signature = match serde_json::from_str(&candidate_str) {
        Ok(s) => s,
        Err(_) => return 0.0,
    };

    let matcher = Matcher::new();
    matcher.score(&query, &candidate)
}

/// Top-K search: find the k most similar signatures to the query.
/// query_json: JSON-serialized Signature
/// candidates_json: JSON array of Signature objects
/// k: number of results to return
/// Returns: JSON array of {signature_index, similarity} objects
#[no_mangle]
pub extern "C" fn topk_search_ffi(
    query_json: *const c_char,
    candidates_json: *const c_char,
    k: usize,
) -> *mut c_char {
    let query_str = match cstr_to_string(query_json) {
        Some(s) => s,
        None => return CString::new("[]").unwrap().into_raw(),
    };
    let candidates_str = match cstr_to_string(candidates_json) {
        Some(s) => s,
        None => return CString::new("[]").unwrap().into_raw(),
    };

    let query: Signature = match serde_json::from_str(&query_str) {
        Ok(s) => s,
        Err(_) => return CString::new("[]").unwrap().into_raw(),
    };
    let candidates: Vec<Signature> = match serde_json::from_str(&candidates_str) {
        Ok(s) => s,
        Err(_) => return CString::new("[]").unwrap().into_raw(),
    };

    let matcher = Matcher::new().with_k(k);
    let results = matcher.top_k(&query, &candidates);

    let output: Vec<serde_json::Value> = results
        .iter()
        .map(|m| {
            serde_json::json!({
                "index": m.index,
                "similarity": m.similarity,
            })
        })
        .collect();

    let json = serde_json::to_string(&output).unwrap_or_else(|_| "[]".to_string());
    CString::new(json).unwrap().into_raw()
}

/// Compute a topology-independent shape hash for a signature.
/// signature_json: JSON-serialized Signature
/// Returns: JSON string with the hash
#[no_mangle]
pub extern "C" fn shape_hash_ffi(
    signature_json: *const c_char,
) -> *mut c_char {
    let sig_str = match cstr_to_string(signature_json) {
        Some(s) => s,
        None => return CString::new("").unwrap().into_raw(),
    };

    let sig: Signature = match serde_json::from_str(&sig_str) {
        Ok(s) => s,
        Err(_) => return CString::new("").unwrap().into_raw(),
    };

    let hash = crate::compute_shape_hash(&sig);
    CString::new(hash).unwrap().into_raw()
}

/// Free a C string allocated by this library.
#[no_mangle]
pub extern "C" fn free_c_string(ptr: *mut c_char) {
    if !ptr.is_null() {
        unsafe {
            drop(CString::from_raw(ptr));
        }
    }
}
