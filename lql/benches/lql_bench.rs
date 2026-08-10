use criterion::{black_box, criterion_group, criterion_main, Criterion};

use loza_lql::compiler::Target;
use loza_lql::lexer::Lexer;
use loza_lql::parser::Parser;
use loza_lql::schema::Schema;
use loza_lql::{compile_to_duckdb, parse};

// ── Query samples ────────────────────────────────────────────────────────────

const SIMPLE_QUERY: &str = r#"from events | where level = "error" | limit 10"#;

const MEDIUM_QUERY: &str = r#"from events
    | where level = "error" and service = "checkout"
    | summarize count(), avg(duration_ms), p95(duration_ms) by service
    | sort count desc
    | limit 20"#;

const COMPLEX_QUERY: &str = r#"from events
    | where (level = "error" or level = "fatal") and service != "gateway"
    | where duration_ms > 1000 and status_code >= 500
    | extend is_slow = (duration_ms > 5000)
    | summarize cnt = count(), avg_dur = avg(duration_ms), p99_dur = p99(duration_ms), distinct_services = dcount(service) by event
    | where cnt > 10
    | sort p99_dur desc
    | limit 50"#;

const LARGE_QUERY: &str = r#"from events
    | where level = "error" or level = "fatal" or level = "warn"
    | where service = "payments-svc" or service = "checkout" or service = "billing-svc" or service = "inventory-svc"
    | where duration_ms > 500 and duration_ms < 30000
    | where status_code >= 400
    | where environment = "production"
    | where region = "us-east-1" or region = "eu-west-1"
    | extend error_bucket = (duration_ms / 1000)
    | extend is_critical = (level = "fatal" and status_code >= 500)
    | summarize total = count(), errors = count(), avg_latency = avg(duration_ms), p50_latency = p50(duration_ms), p95_latency = p95(duration_ms), p99_latency = p99(duration_ms), max_latency = max(duration_ms), min_latency = min(duration_ms), unique_hosts = dcount(host), unique_services = dcount(service) by event
    | extend error_rate = (errors * 100.0 / total)
    | where total > 100
    | sort p99_latency desc
    | project event, total, errors, error_rate, avg_latency, p50_latency, p95_latency, p99_latency, max_latency, unique_hosts
    | limit 100"#;

const TIMESERIES_QUERY: &str = r#"from events
    | where level = "error" and service = "api-gateway"
    | timeseries 5m"#;

const PROJECT_EXTEND_QUERY: &str = r#"from events
    | where service = "checkout"
    | project service, level, duration_ms, status_code, message
    | extend is_error = (level = "error")
    | extend is_slow = (duration_ms > 3000)
    | where is_error or is_slow
    | sort duration_ms desc
    | limit 50"#;

const FUNCTION_HEAVY_QUERY: &str = r#"from events
    | where tolower(service) = "payments" and strlen(message) > 10
    | where not isempty(error_message)
    | extend upper_svc = toupper(service)
    | extend rounded = round(duration_ms, 2)
    | extend abs_val = abs(duration_ms - 1000)
    | summarize count(), avg(duration_ms) by service
    | sort count desc
    | limit 20"#;

const INVALID_QUERY: &str = r#"from events | where @#$%"#;

const SYNTAX_ERROR_QUERY: &str = r#"from events | where level = "#;

// ── LQL benchmarks ───────────────────────────────────────────────────────────

fn bench_lexer(c: &mut Criterion) {
    c.bench_function("lexer_simple", |b| {
        b.iter(|| {
            let tokens = Lexer::new(black_box(SIMPLE_QUERY)).tokenize();
            black_box(tokens);
        })
    });

    c.bench_function("lexer_complex", |b| {
        b.iter(|| {
            let tokens = Lexer::new(black_box(COMPLEX_QUERY)).tokenize();
            black_box(tokens);
        })
    });

    c.bench_function("lexer_large", |b| {
        b.iter(|| {
            let tokens = Lexer::new(black_box(LARGE_QUERY)).tokenize();
            black_box(tokens);
        })
    });
}

fn bench_parser(c: &mut Criterion) {
    c.bench_function("parser_simple", |b| {
        let tokens = Lexer::new(SIMPLE_QUERY).tokenize().unwrap();
        b.iter(|| {
            let mut parser = Parser::new(black_box(tokens.clone()));
            let pipeline = parser.parse();
            black_box(pipeline);
        })
    });

    c.bench_function("parser_complex", |b| {
        let tokens = Lexer::new(COMPLEX_QUERY).tokenize().unwrap();
        b.iter(|| {
            let mut parser = Parser::new(black_box(tokens.clone()));
            let pipeline = parser.parse();
            black_box(pipeline);
        })
    });

    c.bench_function("parser_large", |b| {
        let tokens = Lexer::new(LARGE_QUERY).tokenize().unwrap();
        b.iter(|| {
            let mut parser = Parser::new(black_box(tokens.clone()));
            let pipeline = parser.parse();
            black_box(pipeline);
        })
    });
}

fn bench_ast_build(c: &mut Criterion) {
    // Full parse pipeline (lexer + parser) to produce AST
    c.bench_function("ast_build_simple", |b| {
        b.iter(|| {
            let pipeline = parse(black_box(SIMPLE_QUERY));
            black_box(pipeline);
        })
    });

    c.bench_function("ast_build_medium", |b| {
        b.iter(|| {
            let pipeline = parse(black_box(MEDIUM_QUERY));
            black_box(pipeline);
        })
    });

    c.bench_function("ast_build_complex", |b| {
        b.iter(|| {
            let pipeline = parse(black_box(COMPLEX_QUERY));
            black_box(pipeline);
        })
    });

    c.bench_function("ast_build_timeseries", |b| {
        b.iter(|| {
            let pipeline = parse(black_box(TIMESERIES_QUERY));
            black_box(pipeline);
        })
    });

    c.bench_function("ast_build_project_extend", |b| {
        b.iter(|| {
            let pipeline = parse(black_box(PROJECT_EXTEND_QUERY));
            black_box(pipeline);
        })
    });

    c.bench_function("ast_build_function_heavy", |b| {
        b.iter(|| {
            let pipeline = parse(black_box(FUNCTION_HEAVY_QUERY));
            black_box(pipeline);
        })
    });
}

fn bench_sql_compilation(c: &mut Criterion) {
    c.bench_function("compile_duckdb_simple", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(SIMPLE_QUERY));
            black_box(sql);
        })
    });

    c.bench_function("compile_duckdb_complex", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(COMPLEX_QUERY));
            black_box(sql);
        })
    });

    c.bench_function("compile_duckdb_large", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(LARGE_QUERY));
            black_box(sql);
        })
    });

    c.bench_function("compile_duckdb_timeseries", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(TIMESERIES_QUERY));
            black_box(sql);
        })
    });

    c.bench_function("compile_duckdb_project_extend", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(PROJECT_EXTEND_QUERY));
            black_box(sql);
        })
    });

    c.bench_function("compile_duckdb_function_heavy", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(FUNCTION_HEAVY_QUERY));
            black_box(sql);
        })
    });

    // ClickHouse compilation
    c.bench_function("compile_clickhouse_complex", |b| {
        b.iter(|| {
            let sql = loza_lql::compile_to_clickhouse(black_box(COMPLEX_QUERY));
            black_box(sql);
        })
    });

    // Compile-only (pre-parsed AST)
    let pipeline = parse(COMPLEX_QUERY).unwrap();
    let schema = Schema::duckdb_default();
    c.bench_function("compile_only_duckdb", |b| {
        b.iter(|| {
            let sql = loza_lql::compiler::compile(
                black_box(&pipeline),
                Target::DuckDB,
                black_box(&schema),
            );
            black_box(sql);
        })
    });

    let pipeline_ch = parse(COMPLEX_QUERY).unwrap();
    let schema_ch = Schema::clickhouse_default();
    c.bench_function("compile_only_clickhouse", |b| {
        b.iter(|| {
            let sql = loza_lql::compiler::compile(
                black_box(&pipeline_ch),
                Target::ClickHouse,
                black_box(&schema_ch),
            );
            black_box(sql);
        })
    });
}

fn bench_invalid_query(c: &mut Criterion) {
    c.bench_function("lexer_invalid_chars", |b| {
        b.iter(|| {
            let tokens = Lexer::new(black_box(INVALID_QUERY)).tokenize();
            black_box(tokens);
        })
    });

    c.bench_function("parser_syntax_error", |b| {
        b.iter(|| {
            // Tokenize first (this succeeds), then parse (this fails)
            if let Ok(tokens) = Lexer::new(black_box(SYNTAX_ERROR_QUERY)).tokenize() {
                let mut parser = Parser::new(tokens);
                let result = parser.parse();
                black_box(result);
            }
        })
    });

    c.bench_function("compile_invalid_query", |b| {
        b.iter(|| {
            let result = compile_to_duckdb(black_box(INVALID_QUERY));
            black_box(result);
        })
    });
}

fn bench_large_query(c: &mut Criterion) {
    // Construct a very large query with many chained where clauses
    let mut large_where = String::from("from events");
    for i in 0..50 {
        large_where.push_str(&format!(r#" | where field_{} = "value_{}""#, i % 10, i));
    }
    large_where.push_str(" | summarize count() by service | sort count desc | limit 100");

    c.bench_function("lexer_50_where_clauses", |b| {
        b.iter(|| {
            let tokens = Lexer::new(black_box(&large_where)).tokenize();
            black_box(tokens);
        })
    });

    c.bench_function("parse_50_where_clauses", |b| {
        let tokens = Lexer::new(&large_where).tokenize().unwrap();
        b.iter(|| {
            let mut parser = Parser::new(black_box(tokens.clone()));
            let pipeline = parser.parse();
            black_box(pipeline);
        })
    });

    c.bench_function("compile_50_where_clauses", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(&large_where));
            black_box(sql);
        })
    });

    // Large summarize with many aggregation functions
    let large_summarize = r#"from events
        | summarize count(), sum(duration_ms), avg(duration_ms), min(duration_ms), max(duration_ms), p50(duration_ms), p95(duration_ms), p99(duration_ms), dcount(service), dcount(host), dcount(trace_id), first(level), last(level) by event, environment, region
        | sort count desc
        | limit 200"#;

    c.bench_function("compile_many_aggregations", |b| {
        b.iter(|| {
            let sql = compile_to_duckdb(black_box(large_summarize));
            black_box(sql);
        })
    });
}

criterion_group!(
    benches,
    bench_lexer,
    bench_parser,
    bench_ast_build,
    bench_sql_compilation,
    bench_invalid_query,
    bench_large_query,
);
criterion_main!(benches);
