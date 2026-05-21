package output

import "context"

type formatKey struct{}
type verboseKey struct{}

func WithFormat(ctx context.Context, format string) context.Context {
	return context.WithValue(ctx, formatKey{}, format)
}

func GetFormat(ctx context.Context) string {
	if v, ok := ctx.Value(formatKey{}).(string); ok {
		return v
	}
	return "text"
}

func ShouldOutputJSON(ctx context.Context) bool {
	return GetFormat(ctx) == "json"
}

func WithVerbose(ctx context.Context) context.Context {
	return context.WithValue(ctx, verboseKey{}, true)
}

func IsVerbose(ctx context.Context) bool {
	v, _ := ctx.Value(verboseKey{}).(bool)
	return v
}
