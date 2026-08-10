use std::time::Duration;

#[test]
fn process_group_timer_and_stopwatch_helpers_work() {
    let mut ctx = loza::start_event(
        None,
        loza::Params::new("checkout.request").with_kind("http"),
    );

    loza::with_process(&mut ctx, "authorize_payment", |handle, event| {
        handle.finish(event, &[loza::string("payment.status", "approved")]);
    });
    loza::with_group(&mut ctx, "payment_flow", |handle, event| {
        handle.finish(event, &[loza::string("phase", "done")]);
    });
    loza::with_timer(&mut ctx, "db.lookup", |handle, event| {
        handle.stop(event, &[loza::string("cache", "miss")]);
    });
    loza::measure(&mut ctx, "measure.wrap", |event| {
        event.append_attr(loza::string("measure", "done"));
    });
    loza::step(&mut ctx, "step.wrap", |event| {
        event.append_attr(loza::string("step", "done"));
    });
    loza::phase(&mut ctx, "phase.wrap", |event| {
        event.append_attr(loza::string("phase", "done"));
    });
    loza::span(&mut ctx, "span.wrap", |event| {
        event.append_attr(loza::string("span", "done"));
    });

    let stopwatch = loza::StopwatchHandle::new();
    std::thread::sleep(Duration::from_millis(1));
    assert!(stopwatch.elapsed().as_millis() >= 1);

    loza::finish(&mut ctx);
    loza::emit(&mut ctx).expect("emit");
    assert!(!ctx.processes.is_empty());
    assert!(!ctx.groups.is_empty());
    assert!(!ctx.timers.is_empty());
}
