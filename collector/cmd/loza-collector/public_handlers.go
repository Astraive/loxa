package main

import "net/http"

func (s *collectorState) HandleIngest(w http.ResponseWriter, r *http.Request) { s.handleIngest(w, r) }
func (s *collectorState) HandleOTLPLogs(w http.ResponseWriter, r *http.Request) {
	s.handleOTLPLogs(w, r)
}
func (s *collectorState) HandleHealth(w http.ResponseWriter, r *http.Request)  { s.handleHealth(w, r) }
func (s *collectorState) HandleReady(w http.ResponseWriter, r *http.Request)   { s.handleReady(w, r) }
func (s *collectorState) HandleVersion(w http.ResponseWriter, r *http.Request) { s.handleVersion(w, r) }
func (s *collectorState) HandleStatus(w http.ResponseWriter, r *http.Request)  { s.handleStatus(w, r) }
func (s *collectorState) HandleValidate(w http.ResponseWriter, r *http.Request) {
	s.handleValidate(w, r)
}
func (s *collectorState) HandleSinks(w http.ResponseWriter, r *http.Request) { s.handleSinks(w, r) }
func (s *collectorState) HandleSink(w http.ResponseWriter, r *http.Request)  { s.handleSink(w, r) }
func (s *collectorState) HandleSinkTest(w http.ResponseWriter, r *http.Request) {
	s.handleSinkTest(w, r)
}
func (s *collectorState) HandleSchemaList(w http.ResponseWriter, r *http.Request) {
	s.handleSchemaList(w, r)
}
func (s *collectorState) HandleSchemaDiff(w http.ResponseWriter, r *http.Request) {
	s.handleSchemaDiff(w, r)
}
func (s *collectorState) HandleSchemaCheck(w http.ResponseWriter, r *http.Request) {
	s.handleSchemaCheck(w, r)
}
func (s *collectorState) HandleSchemaPublish(w http.ResponseWriter, r *http.Request) {
	s.handleSchemaPublish(w, r)
}
func (s *collectorState) HandleQuery(w http.ResponseWriter, r *http.Request) { s.handleQuery(w, r) }
func (s *collectorState) HandlePIIAudit(w http.ResponseWriter, r *http.Request) {
	s.handlePIIAudit(w, r)
}
func (s *collectorState) HandlePolicyValidate(w http.ResponseWriter, r *http.Request) {
	s.handlePolicyValidate(w, r)
}
func (s *collectorState) HandleRetentionApply(w http.ResponseWriter, r *http.Request) {
	s.handleRetentionApply(w, r)
}
func (s *collectorState) HandleKeyCreate(w http.ResponseWriter, r *http.Request) {
	s.handleKeyCreate(w, r)
}
func (s *collectorState) HandleKeyRevoke(w http.ResponseWriter, r *http.Request) {
	s.handleKeyRevoke(w, r)
}
func (s *collectorState) HandleKeyRotate(w http.ResponseWriter, r *http.Request) {
	s.handleKeyRotate(w, r)
}
func (s *collectorState) HandleDeleteEvents(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteEvents(w, r)
}
func (s *collectorState) HandleDLQList(w http.ResponseWriter, r *http.Request) { s.handleDLQList(w, r) }
func (s *collectorState) HandleDLQReplayAll(w http.ResponseWriter, r *http.Request) {
	s.handleDLQReplayAll(w, r)
}
func (s *collectorState) HandleDLQShow(w http.ResponseWriter, r *http.Request) { s.handleDLQShow(w, r) }
func (s *collectorState) HandleDLQReplay(w http.ResponseWriter, r *http.Request) {
	s.handleDLQReplay(w, r)
}
func (s *collectorState) HandleDLQDelete(w http.ResponseWriter, r *http.Request) {
	s.handleDLQDelete(w, r)
}
func (s *collectorState) HandleTail(w http.ResponseWriter, r *http.Request) { s.handleTail(w, r) }
func (s *collectorState) HandleReplay(w http.ResponseWriter, r *http.Request) {
	s.handleReplay(w, r)
}
func (s *collectorState) HandleBlueprintPublish(w http.ResponseWriter, r *http.Request) {
	s.handleBlueprintPublish(w, r)
}
func (s *collectorState) HandleBlueprintList(w http.ResponseWriter, r *http.Request) {
	s.handleBlueprintList(w, r)
}
