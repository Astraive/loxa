from ...core.attr import String

def EnrichTrace(trace_id: str = "", span_id: str = ""):
    attrs = []
    if trace_id:
        attrs.append(String("trace_id", trace_id))
    if span_id:
        attrs.append(String("span_id", span_id))
    return attrs
