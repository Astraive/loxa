# Pitfalls

Do not send heavy backend credentials from applications. Rust SDK delivery
should use stdout, file, memory, noop, or collector HTTP batch sinks only.

