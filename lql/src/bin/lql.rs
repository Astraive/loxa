use std::io::{self, Read};

fn main() {
    let args: Vec<String> = std::env::args().collect();

    if args.len() < 2 {
        eprintln!("Loza Query Language (LQL) compiler");
        eprintln!();
        eprintln!("Usage:");
        eprintln!("  lql compile <query>          Compile LQL to DuckDB SQL");
        eprintln!("  lql compile-ch <query>       Compile LQL to ClickHouse SQL");
        eprintln!("  lql check <query>            Validate LQL syntax and fields");
        eprintln!("  lql fields                   List known event fields");
        eprintln!();
        eprintln!("Examples:");
        eprintln!("  lql compile 'from events | where level = \"error\" | limit 10'");
        eprintln!("  lql compile 'from events | summarize count() by service | sort count desc'");
        eprintln!("  echo 'from events | limit 5' | lql compile");
        std::process::exit(1);
    }

    let command = args[1].as_str();

    match command {
        "compile" | "compile-ch" | "check" => {
            let query = if args.len() > 2 {
                args[2..].join(" ")
            } else {
                let mut buf = String::new();
                if let Err(err) = io::stdin().read_to_string(&mut buf) {
                    eprintln!("error: failed to read stdin: {}", err);
                    std::process::exit(1);
                }
                buf
            };

            let query = query.trim();
            if query.is_empty() {
                eprintln!("error: empty query");
                std::process::exit(1);
            }

            match command {
                "compile" => match loza_lql::compile_to_duckdb(query) {
                    Ok(sql) => println!("{}", sql),
                    Err(e) => {
                        eprintln!("error: {}", e);
                        std::process::exit(1);
                    }
                },
                "compile-ch" => match loza_lql::compile_to_clickhouse(query) {
                    Ok(sql) => println!("{}", sql),
                    Err(e) => {
                        eprintln!("error: {}", e);
                        std::process::exit(1);
                    }
                },
                "check" => match loza_lql::validate_query(query) {
                    Ok(()) => {
                        println!("ok");
                    }
                    Err(e) => {
                        eprintln!("error: {}", e);
                        std::process::exit(1);
                    }
                },
                _ => unreachable!(),
            }
        }
        "fields" => {
            let fields = loza_lql::known_fields();
            for f in fields {
                println!("{}", f);
            }
        }
        "--help" | "-h" => {
            println!("Loza Query Language (LQL) compiler");
            println!();
            println!("Usage: lql <command> [query]");
            println!();
            println!("Commands:");
            println!("  compile <query>       Compile LQL to DuckDB SQL");
            println!("  compile-ch <query>    Compile LQL to ClickHouse SQL");
            println!("  check <query>         Validate LQL syntax and fields");
            println!("  fields                List known event fields");
        }
        "--version" | "-v" => {
            println!("loza-lql {}", env!("CARGO_PKG_VERSION"));
        }
        _ => {
            eprintln!("unknown command: {}", command);
            eprintln!("Run 'lql --help' for usage.");
            std::process::exit(1);
        }
    }
}
