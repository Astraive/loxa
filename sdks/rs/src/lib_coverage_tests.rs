#[cfg(test)]
mod coverage_tests {
    use super::super::*;
    use serde_json::json;
    use std::time::Duration;

    #[test]
    fn public_factories_attributes_and_aliases_are_exercised() {
        let cfg = Config::test("coverage");
        let _ = New(cfg.clone());
        let _ = NewWith(vec![WithService("coverage")]);
        let _ = TryNew(cfg.clone()).unwrap();
        let _ = NewClient(cfg.clone());
        let _ = Configure(cfg.clone());
        let _ = crate::configure(cfg.clone()).unwrap();
        let _ = reset();
        let _ = Reset();
        let _ = crate::default();
        let _ = Default("coverage");
        let _ = Dev("coverage");
        let _ = Production("coverage");
        let _ = Test("coverage");
        let _ = new_logger(cfg.clone());
        let _ = new_with(vec![WithVersion("v1")]);
        let _ = try_new_logger(cfg.clone()).unwrap();
        let _ = dev("coverage");
        let _ = production("coverage");
        let _ = test("coverage");

        let _ = StartHTTPEvent("GET", "/health");
        let _ = StartJobEvent("job");
        let _ = StartQueueEvent("queue");
        let _ = StartCLIEvent("cli");
        let _ = StartCronEvent("cron");
        let _ = from_request("GET", "/", "/", vec![]);
        let _ = FromRequest("POST", "/items", "/items", vec![String("x", "y")]);

        let mut event = StartEvent(None, Params::new("coverage"));
        Append(&mut event, String("one", "1"));
        Enrich(&mut event, vec![Int("two", 2), Bool("three", true)]);
        Set(&mut event, "four", json!(4));
        let mut fields = serde_json::Map::new();
        fields.insert("five".into(), json!(5));
        Merge(&mut event, fields);
        assert_eq!(Get(&event, "one"), Some(&json!("1")));
        assert!(GetGroup(&event, "missing").is_none());
        Checkpoint(&mut event, "checkpoint");
        CheckpointWithAttrs(&mut event, "checkpoint2", &[String("ok", "yes")]);
        Delete(&mut event, "one");
        append(&mut event, string("alias", "value"));
        enrich(&mut event, vec![int("alias_int", 1)]);
        set(&mut event, "alias_set", true);
        merge(
            &mut event,
            serde_json::Map::from_iter([("alias_merge".into(), json!(true))]),
        );
        checkpoint(&mut event, "alias_checkpoint");
        checkpoint_with_attrs(&mut event, "alias_checkpoint2", &[string("x", "y")]);
        assert!(has_event(&event));
        assert!(from_context(&event).is_some());
        assert!(event_id(&event).is_some());
        assert!(request_id_from_context(&event).is_some());
        assert!(trace_id_from_context(&event).is_none());
        assert!(span_id_from_context(&event).is_none());
        let mut request = crate::core::client::HTTPRequest {
            method: "GET".into(),
            url: "/".into(),
            headers: std::collections::BTreeMap::new(),
            body: None,
        };
        InjectHTTPHeaders(&mut request, &event);
        let mut headers = std::collections::BTreeMap::new();
        InjectHTTPHeadersFromCarrier(
            &ContextCarrier {
                trace_id: Some("0123456789abcdef0123456789abcdef".into()),
                span_id: Some("0123456789abcdef".into()),
                request_id: Some("request".into()),
                baggage: std::collections::BTreeMap::from([("k".into(), "v".into())]),
                ..ContextCarrier::default()
            },
            &mut headers,
        );
        let extracted = std::collections::BTreeMap::from([
            ("x-trace-id".into(), "trace".into()),
            ("x-span-id".into(), "span".into()),
            ("x-request-id".into(), "request".into()),
        ]);
        assert_eq!(
            ExtractHTTPHeaders(&extracted),
            (
                Some("trace".into()),
                Some("span".into()),
                Some("request".into())
            )
        );
        assert_eq!(ExtractHTTPHeaders(&headers).2, Some("request".into()));
        assert_eq!(
            headers.get("baggage").map(std::string::String::as_str),
            Some("k=v")
        );
        Finish(&mut event);
        let _ = EmitEvent(&mut event);
        let _ = emit(&mut event);
        FinishError(&mut event, "error");
        finish_error(&mut event, "alias-error");
        Flush();
        flush();
        Shutdown();
        shutdown();

        let now = time::OffsetDateTime::UNIX_EPOCH;
        let attrs = vec![
            String("string", "value"),
            Int("int", -1),
            Int64("int64", -2),
            Uint64("uint", 3),
            Float64("float64", 1.5),
            Float("float", 1.5),
            Bool("bool", true),
            Time("time", now),
            Duration("duration", Duration::from_millis(2)),
            Any("any", json!({"x": 1})),
            Null("null"),
            Group("group", vec![String("nested", "yes")]),
            SensitiveString("sensitive", "secret"),
            MarkSensitive(String("marked", "secret")),
            HashString("hashed", "secret"),
            UserID("u"),
            TenantID("t"),
            RequestID("r"),
            TraceID("tr"),
            SpanID("sp"),
            ServiceName("svc"),
            Environment("prod"),
            Region("us"),
            Version("v"),
            ErrorAttr("err"),
            StatusCode(200),
            Method("GET"),
            Path("/"),
            Route("/"),
            Message("msg"),
            WorkspaceID("w"),
            OrganizationID("o"),
            SessionID("s"),
            FeatureFlag("feature", json!("on")),
            FeatureFlagBool("bool", true),
            Experiment("exp", "a"),
            OrderID("order"),
            CartID("cart"),
            ProductID("product"),
            CustomerID("customer"),
            Plan("pro"),
            Currency("USD"),
            Amount(2.5),
            Country("US"),
            Device("mobile"),
            Platform("ios"),
            AppVersion("1"),
            ErrorType("Type"),
            ErrorCode("CODE"),
            ErrorMessage("message"),
            ErrorStack("stack"),
            Retryable(true),
            PaymentID("payment"),
            SubscriptionID("subscription"),
            InvoiceID("invoice"),
            JobID("job"),
            MessageID("message"),
            CorrelationID("correlation"),
            CommitSHA("sha"),
            Release("release"),
            Money(1.0),
            Percent(0.5),
            Bytes(10),
            HTTPStatus(201),
            Bucket("bucket"),
            Tags(vec!["a", "b"]),
            Masked("password"),
            List("list", vec![1, 2]),
            Map("map", serde_json::Map::from_iter([("x".into(), json!(1))])),
            Enum("enum", "value", vec!["value"]),
            ID("id", "value"),
            Hash("hash", "value"),
            Redacted("redacted"),
            AccountID("account"),
            URL("https://example.test"),
            DeploymentID("deployment"),
            HTTPRoute("/route"),
            HTTPMethod("get"),
            HTTPPath("/path"),
            HTTPUserAgent("agent"),
            HTTPUserAgent("a".repeat(513)),
            HTTPReferer("https://example.test/path?q=1"),
            HTTPRequest("POST", "/request"),
            HTTPResponse(204),
            EmailHash("user@example.test"),
            IPHash("127.0.0.1"),
            CheckoutCartItemCount(1),
            CheckoutCartTotal(2.0),
            CheckoutPaymentMethod("card"),
            CheckoutStatus("ok"),
            PaymentProvider("stripe"),
            PaymentMethod("card"),
            PaymentIntentID("intent"),
            PaymentFailureCode("declined"),
            PaymentRetryAttempt(1),
            BillingPlan("pro"),
            BillingSubscriptionID("sub"),
            BillingInvoiceID("inv"),
            BillingAmount(2.0),
            BillingInterval("month"),
            AgentName("agent"),
            AgentProvider("provider"),
            AgentModel("model"),
            AgentRunType("completion"),
            AgentToolName("tool"),
            AgentToolOutcome("ok"),
            AgentInputTokens(1),
            AgentOutputTokens(2),
            AgentCost(0.1),
            RAGIndex("index"),
            RAGEmbeddingModel("model"),
            RAGChunksRetrieved(1),
            RAGTopScore(0.9),
            RAGQueryHash("hash"),
            RAGCitationCount(1),
            RAGRetrievalLatency(3),
        ];
        assert!(attrs.iter().all(|attr| !attr.key.is_empty()));
        let _ = attrs;

        let _ = user_id("u");
        let _ = tenant_id("t");
        let _ = request_id("r");
        let _ = trace_id("tr");
        let _ = span_id("sp");
        let _ = service_name("svc");
        let _ = environment("prod");
        let _ = region("us");
        let _ = version("v");
        let _ = error_attr("e");
        let _ = status_code(200);
        let _ = method("GET");
        let _ = path("/");
        let _ = route("/");
        let _ = message("m");
        let _ = workspace_id("w");
        let _ = organization_id("o");
        let _ = session_id("s");
        let _ = feature_flag("f", json!(true));
        let _ = feature_flag_bool("f", true);
        let _ = experiment("e", "v");
        let _ = order_id("o");
        let _ = cart_id("c");
        let _ = product_id("p");
        let _ = customer_id("c");
        let _ = plan("p");
        let _ = currency("USD");
        let _ = amount(1.0);
        let _ = country("US");
        let _ = device("d");
        let _ = platform("p");
        let _ = app_version("v");
        let _ = error_type("t");
        let _ = error_code("c");
        let _ = error_message("m");
        let _ = error_stack("s");
        let _ = retryable(false);
        let _ = string("s", "v");
        let _ = int("i", 1);
        let _ = int64("i", 1);
        let _ = uint64("u", 1);
        let _ = float("f", 1.0);
        let _ = float64("f", 1.0);
        let _ = bool("b", true);
        let _ = time("t", now);
        let _ = duration("d", Duration::from_secs(1));
        let _ = any("a", json!(null));
        let _ = json("j", json!({}));
        let _ = null("n");
        let _ = group_attr("g", vec![string("x", "y")]);
        let _ = sensitive_string("s", "v");
        let _ = mark_sensitive(string("s", "v"));
        let _ = hash_string("h", "v");
    }
}
