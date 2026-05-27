package core

func partitionStructuredAttrs(attrs map[string]any) (http map[string]any, user map[string]any, tenant map[string]any, resource map[string]any, extra map[string]any) {
	if len(attrs) == 0 {
		return nil, nil, nil, nil, nil
	}
	extra = cloneAnyMap(attrs)
	if extra == nil {
		extra = map[string]any{}
	}

	http = takeStructuredGroup(extra, "http")
	user = takeStructuredGroup(extra, "user")
	tenant = takeStructuredGroup(extra, "tenant")
	resource = takeStructuredGroup(extra, "resource")
	return http, user, tenant, resource, extra
}

func takeStructuredGroup(attrs map[string]any, key string) map[string]any {
	group, _ := attrs[key].(map[string]any)
	delete(attrs, key)
	return cloneAnyMap(group)
}

func mergeHTTPPayload(base map[string]any, method, path, route string, statusCode int) map[string]any {
	if base == nil {
		base = map[string]any{}
	} else {
		base = cloneAnyMap(base)
	}
	if method != "" {
		base["method"] = method
	}
	if path != "" {
		base["path"] = path
	}
	if route != "" {
		base["route"] = route
	}
	if statusCode != 0 {
		base["status_code"] = statusCode
	}
	if len(base) == 0 {
		return nil
	}
	return base
}

func isNonEmptyMap(value map[string]any) bool {
	return len(value) > 0
}
