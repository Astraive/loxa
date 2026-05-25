use std::time::Duration;

#[test]
fn process_group_timer_and_stopwatch_helpers_work() {
    let mut ctx = loxa::start_event(
        None,
        loxa::Params::new("checkout.request").with_kind("http"),
    );

    loxa::with_process(&mut ctx, "authorize_payment", |handle, event| {
        handle.finish(event, &[loxa::string("payment.status", "approved")]);
    });
    loxa::with_group(&mut ctx, "payment_flow", |handle, event| {
        handle.finish(event, &[loxa::string("phase", "done")]);
    });
    loxa::with_timer(&mut ctx, "db.lookup", |handle, event| {
        handle.stop(event, &[loxa::string("cache", "miss")]);
    });
    loxa::measure(&mut ctx, "measure.wrap", |event| {
        event.append_attr(loxa::string("measure", "done"));
    });
    loxa::step(&mut ctx, "step.wrap", |event| {
        event.append_attr(loxa::string("step", "done"));
    });
    loxa::phase(&mut ctx, "phase.wrap", |event| {
        event.append_attr(loxa::string("phase", "done"));
    });
    loxa::span(&mut ctx, "span.wrap", |event| {
        event.append_attr(loxa::string("span", "done"));
    });

    let stopwatch = loxa::StopwatchHandle::new();
    std::thread::sleep(Duration::from_millis(1));
    assert!(stopwatch.elapsed().as_millis() >= 1);

    loxa::finish(&mut ctx);
    loxa::emit(&mut ctx).expect("emit");
    assert!(!ctx.processes.is_empty());
    assert!(!ctx.groups.is_empty());
    assert!(!ctx.timers.is_empty());
}
