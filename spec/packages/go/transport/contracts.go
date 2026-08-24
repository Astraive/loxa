package contracts

type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

type GraphQLResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message   string     `json:"message"`
	Locations []Location `json:"locations,omitempty"`
}

type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type WebSocketRequest struct {
	Action        string                 `json:"action"`
	Query         string                 `json:"query,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
	Event         map[string]any         `json:"event,omitempty"`
	IncidentID    string                 `json:"incident_id,omitempty"`
	Service       string                 `json:"service,omitempty"`
	Mode          string                 `json:"mode,omitempty"`
	Depth         int                    `json:"depth,omitempty"`
}

type WebSocketResponse struct {
	Type  string      `json:"type,omitempty"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}
