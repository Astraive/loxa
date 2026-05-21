from loxa import Config, Logger, Params


def main() -> None:
    logger = Logger(Config.production(service="demo"))
    ctx = logger.start_event(Params(event="demo.run", method="CLI", path="examples/basic.py"))
    logger.enrich(ctx, component="example", answer=42)
    logger.finish(ctx, outcome="success", status_code=0)
    logger.emit(ctx)


if __name__ == "__main__":
    main()
