## cmd/mock-runtime/main.go
  1:package main
  16:type Envelope struct {
  25:func main() {
  90:func generateResponse(input, flavor string) string {
  108:func splitChunks(text string) []string {
  128:func taskText(payload map[string]any) string {
  139:func env(key, fallback string) string {

## cmd/shclop-runtime/main.go
  1:package main
  15:func main() {
  100:func adapterForRuntime(runtimeName string) claw.Adapter {
  115:func taskText(payload map[string]any) string {
  120:func clawEventToEnvelope(event claw.Event, task gateway.Envelope, seq int) gateway.Envelope {
  148:func isTerminalEnvelope(eventType string) bool {
  152:func env(key, fallback string) string {
  159:func runtimeTokenFromEnv() string {

## cmd/shclop/main.go
  1:package main
  12:func main() {

## internal/api/oidc.go
  1:package api
  21:const oidcStateCookieName = "shclop_oidc_state"
  23:func (s *Server) handleListAuthProviders(w http.ResponseWriter, r *http.Request) {
  45:func (s *Server) handleOIDCRoute(w http.ResponseWriter, r *http.Request) {
  72:func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request, providerName string) {
  136:func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request, providerName string) {
  260:func (s *Server) linkOrCreateUser(ctx context.Context, providerName, subject, email, displayName string) (domain.User, error) {
  298:func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
  330:func (s *Server) handleAuthSettings(w http.ResponseWriter, r *http.Request) {
  341:func (s *Server) handleGetAuthSettings(w http.ResponseWriter, r *http.Request) {
  370:func (s *Server) handlePatchAuthSettings(w http.ResponseWriter, r *http.Request) {
  428:func (s *Server) handleAdminListIdentities(w http.ResponseWriter, r *http.Request, userID string) {
  450:func (s *Server) handleAdminLinkIdentity(w http.ResponseWriter, r *http.Request, userID string) {
  496:func (s *Server) handleAdminUnlinkIdentity(w http.ResponseWriter, r *http.Request, userID, providerName, subject string) {
  524:type userIdentityJSON struct {
  534:func toIdentityJSON(ids []domain.UserIdentity) []userIdentityJSON {
  550:func sanitizeReturnTo(returnTo string) string {

## internal/api/server.go
  1:package api
  39:var wsUpgrader = websocket.Upgrader{CheckOrigin: sameOriginOrNoOrigin}
  41:func sameOriginOrNoOrigin(r *http.Request) bool {
  53:type Server struct {
  74:type MetricsCollectors struct {
  89:func newMetricsCollectors() *MetricsCollectors {
  155:type activityEntry struct {
  164:func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
  320:func (s *Server) requireBootstrapPassword() error {
  330:func (s *Server) bootstrapAdmin() {
  374:func requestContext() requestCtx {
  378:type requestCtx struct{}
  380:func (requestCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
  381:func (requestCtx) Done() <-chan struct{}       { return nil }
  382:func (requestCtx) Err() error                  { return nil }
  383:func (requestCtx) Value(key any) any           { return nil }
  385:func sandboxProviderFromConfig(cfg config.Config) (sandbox.RuntimeProvider, error) {
  417:func (s *Server) ListenAndServe() error {
  432:func (s *Server) Handler() http.Handler {
  436:func (s *Server) withMetrics(next http.Handler) http.Handler {
  447:type statusWriter struct {
  452:func (sw *statusWriter) WriteHeader(code int) {
  457:func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
  464:func (s *Server) routes() http.Handler {
  517:func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
  525:func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
  539:func (s *Server) handleMetrics() http.Handler {
  550:func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
  594:func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
  608:func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
  619:func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
  656:func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
  717:func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
  733:func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  755:func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  901:func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  939:func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  973:func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
  981:func (s *Server) handleIntegration(w http.ResponseWriter, r *http.Request) {
  1006:func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
  1020:func (s *Server) handleConnectIntegration(w http.ResponseWriter, r *http.Request, providerID string) {
  1070:func (s *Server) handleDisconnectIntegration(w http.ResponseWriter, r *http.Request, providerID string) {
  1086:func (s *Server) handleAgentIntegration(w http.ResponseWriter, r *http.Request, agentID, providerID string) {
  1153:func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
  1164:func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
  1184:func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
  1234:func (s *Server) handleAdminUser(w http.ResponseWriter, r *http.Request) {
  1267:func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request, targetUserID string) {
  1306:func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
  1315:func (s *Server) handleListEnabledModels(w http.ResponseWriter, r *http.Request) {
  1370:func (s *Server) fetchLiteLLMModels(ctx context.Context, baseURL, apiKey string) (map[string]bool, error) {
  1416:func (s *Server) handleAdminModels(w http.ResponseWriter, r *http.Request) {
  1427:func (s *Server) handleAdminListModels(w http.ResponseWriter, r *http.Request) {
  1447:func (s *Server) handleAdminCreateModel(w http.ResponseWriter, r *http.Request) {
  1480:func (s *Server) handleAdminModel(w http.ResponseWriter, r *http.Request) {
  1494:func (s *Server) handleAdminUpdateModel(w http.ResponseWriter, r *http.Request, modelID string) {
  1527:func (s *Server) handleAdminLLMGateway(w http.ResponseWriter, r *http.Request) {
  1538:func (s *Server) handleAdminGetLLMGateway(w http.ResponseWriter, r *http.Request) {
  1555:func (s *Server) handleAdminUpdateLLMGateway(w http.ResponseWriter, r *http.Request) {
  1585:func (s *Server) handleAdminPlugins(w http.ResponseWriter, r *http.Request) {
  1596:func (s *Server) handleAdminPlugin(w http.ResponseWriter, r *http.Request) {
  1613:func (s *Server) handleAdminListPlugins(w http.ResponseWriter, r *http.Request) {
  1633:func (s *Server) handleAdminUpsertPlugin(w http.ResponseWriter, r *http.Request, pathID string) {
  1692:func (s *Server) handleAdminDeletePlugin(w http.ResponseWriter, r *http.Request, pluginID string) {
  1713:func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
  1749:func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
  1761:func (s *Server) recordActivity(eventType, actorID, agentID, message string, details map[string]any) {
  1774:func (s *Server) activitySnapshot() []activityEntry {
  1780:func (s *Server) activityForUser(user domain.User) []activityEntry {
  1796:func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
  1882:func (s *Server) handleRuntimeWebSocket(w http.ResponseWriter, r *http.Request) {
  1928:func (s *Server) validRuntimeToken(agentID, secret string) bool {
  1936:func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
  1958:func (s *Server) requireUserFromRequest(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
  1981:func (s *Server) enforceCurrentUser(w http.ResponseWriter, r *http.Request, cached domain.User) (domain.User, bool) {
  1994:func chatEventResponse(event gateway.Envelope) map[string]any {
  2009:func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
  2016:func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
  2022:func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
  2050:func writeJSON(w http.ResponseWriter, status int, value any) error {
  2063:func methodNotAllowed(w http.ResponseWriter, allow string) {
  2068:func randomSecret() (string, error) {
  2076:func randomHexID() string {

## internal/auth/auth.go
  1:package auth
  15:type PasswordHasher interface {
  21:type UserStore interface {
  26:type Service struct {
  33:func NewService(store UserStore, hasher PasswordHasher) *Service {
  41:func (s *Service) Login(ctx context.Context, username, password string) (domain.User, string, error) {
  71:func (s *Service) Resolve(token string) (domain.User, bool) {
  79:func (s *Service) IssueToken(user domain.User) (string, error) {
  91:func (s *Service) Revoke(token string) bool {
  101:func tokenID() (string, error) {

## internal/auth/cookiecodec.go
  1:package auth
  17:var ErrExpired = errors.New("oidc state cookie expired")
  18:var ErrInvalidCookie = errors.New("oidc state cookie invalid")
  20:type OIDCStateCookie struct {
  29:type CookieCodec struct {
  34:func NewCookieCodec(rawKey []byte) (*CookieCodec, error) {
  49:func NewCookieCodecFromConfig(configKey string) (*CookieCodec, error) {
  56:func (c *CookieCodec) SetClock(fn func() time.Time) {
  60:func (c *CookieCodec) Encode(payload OIDCStateCookie) (string, error) {
  84:func (c *CookieCodec) Decode(encoded string) (OIDCStateCookie, error) {

## internal/claw/adapter.go
  1:package claw
  5:type Task struct {
  10:type EventType string
  12:const (
  19:type Event struct {
  26:type Adapter interface {
  30:type DemoAdapter struct{ Flavor string }
  32:func (a DemoAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

## internal/claw/nanoclaw.go
  1:package claw
  16:type NanoclawAdapter struct{}
  19:type OpenclawAdapter struct{}
  21:func (a NanoclawAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
  36:func (a OpenclawAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
  54:func writeNanoclawConfig() error {
  105:func buildSystemPrompt() string {
  134:func nanoclawConfigDir() string {
  150:func testWritable(dir string) bool {
  164:func nanoclawEnv() []string {

## internal/claw/openai.go
  1:package claw
  21:type OpenAIAdapter struct{}
  23:const systemPrompt = `You are an AI agent running inside a Linux container. You have full shell access via the bash tool.
  37:var bashTool = map[string]any{
  55:type chatMessage struct {
  63:type toolCall struct {
  72:func (a OpenAIAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
  231:func runBash(ctx context.Context, command string) (string, error) {

## internal/claw/subprocess.go
  1:package claw
  12:type SubprocessAdapter struct {
  18:func (a SubprocessAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

## internal/config/config.go
  1:package config
  10:type IdPProviderConfig struct {
  23:type Config struct {
  75:func Default() (Config, error) {
  132:func (c Config) Validate() error {
  156:func parseIdPProviders() ([]IdPProviderConfig, error) {
  208:func parseScopes(raw string) []string {
  226:func env(key, fallback string) string {
  233:func envBool(key string, fallback bool) bool {

## internal/controllers/mcpserver_controller.go
  1:package controllers
  24:const fieldManager = "shclop-mcp-controller"
  28:type MCPServerController struct {
  38:func NewMCPServerController(client *k8s.Client, informer cache.SharedIndexInformer, namespace string, logger *slog.Logger) *MCPServerController {
  52:func (c *MCPServerController) Run(ctx context.Context, workers int) error {
  93:func (c *MCPServerController) processNextItem(ctx context.Context) bool {
  112:func (c *MCPServerController) reconcile(ctx context.Context, key string) error {
  153:func resourceName(crName string) string {
  159:func selectorLabels(crName string) map[string]string {
  166:func (c *MCPServerController) applyDeployment(ctx context.Context, cr *k8s.MCPServer) error {
  240:func (c *MCPServerController) applyService(ctx context.Context, cr *k8s.MCPServer) error {
  263:func (c *MCPServerController) applyNetworkPolicy(ctx context.Context, cr *k8s.MCPServer) error {
  313:func (c *MCPServerController) updateStatus(ctx context.Context, cr *k8s.MCPServer) {
  338:func (c *MCPServerController) cleanup(ctx context.Context, ns, crName string) error {

## internal/domain/domain.go
  1:package domain
  6:type PluginManifest struct {
  17:type IntegrationConnection struct {
  33:type AgentIntegration struct {
  45:type IntegrationSummary struct {
  51:type ProviderSummary struct {
  66:type FormFieldSummary struct {
  75:type MCPServerSummary struct {
  81:type ConnectionMetadata struct {
  90:type AgentBindingSummary struct {
  97:type User struct {
  106:type Agent struct {
  120:type LLMModel struct {
  129:type LLMGatewaySettings struct {
  137:type AdminOverview struct {
  143:type AdminRuntimeConfig struct {
  150:type AdminObservability struct {
  156:type AdminHealthStatus struct {
  162:type CreateAgentInput struct {
  170:type Message struct {
  180:type AuthMode string
  182:const (
  189:type UserIdentity struct {
  200:type IdPProviderSummary struct {
  209:type AuthSettings struct {

## internal/gateway/envelope.go
  1:package gateway
  3:type Envelope struct {

## internal/gateway/mock_runtime.go
  1:package gateway
  3:type MockRuntime struct{}
  5:func (MockRuntime) Respond(agentID, sessionID, messageID, text string) []Envelope {

## internal/gateway/registry.go
  1:package gateway
  10:var ErrRuntimeNotConnected = errors.New("runtime not connected")
  12:type RuntimeRegistry struct {
  18:type waiterKey struct {
  23:type waiter struct {
  30:func newWaiter() *waiter {
  34:func (w *waiter) cancel() {
  38:func (w *waiter) deliver(event Envelope) bool {
  49:func (w *waiter) fail(event Envelope) {
  60:type RuntimeConnection struct {
  66:func NewRuntimeRegistry() *RuntimeRegistry {
  70:func (r *RuntimeRegistry) Register(agentID string, conn *websocket.Conn) {
  81:func (r *RuntimeRegistry) Unregister(agentID string, conn *websocket.Conn) {
  90:func (r *RuntimeRegistry) SendTask(agentID string, task Envelope) (<-chan Envelope, func(), error) {
  114:func (r *RuntimeRegistry) Dispatch(agentID string, conn *websocket.Conn, event Envelope) {
  134:func (r *RuntimeRegistry) failWaitersLocked(agentID, reason string) {
  144:func (r *RuntimeRegistry) removeWaiter(key waiterKey) {

## internal/identity/mockyaml.go
  1:package identity
  13:var ErrInvalidCredentials = errors.New("invalid credentials")
  15:type MockYAMLProvider struct {
  19:type MockYAMLUserSummary struct {
  29:type mockYAMLConfig struct {
  33:type mockYAMLUser struct {
  43:func NewMockYAMLProvider(path string) (*MockYAMLProvider, error) {
  69:func (p *MockYAMLProvider) Name() string { return "mock-yaml" }
  71:func (p *MockYAMLProvider) Users() []MockYAMLUserSummary {
  91:func (p *MockYAMLProvider) Authenticate(ctx context.Context, request AuthRequest) (Identity, error) {
  117:type StaticOrganizationMapper struct{}
  119:func (StaticOrganizationMapper) Map(ctx context.Context, identity Identity) (MappedPrincipal, error) {
  143:func splitClaim(value string) []string {

## internal/identity/oidc.go
  1:package identity
  13:type OIDCProviderConfig struct {
  27:type OIDCProvider struct {
  35:func NewOIDCProvider(ctx context.Context, cfg OIDCProviderConfig) (*OIDCProvider, error) {
  80:func (p *OIDCProvider) Name() string { return p.Config.Name }
  84:func (p *OIDCProvider) Authenticate(_ context.Context, _ AuthRequest) (Identity, error) {
  90:func (p *OIDCProvider) IdentityFromIDToken(idToken *oidc.IDToken) (Identity, error) {
  122:func stringClaim(raw map[string]any, key string) (string, error) {
  136:func coerceGroups(v any) []string {

## internal/identity/provider.go
  1:package identity
  5:type AuthRequest struct {
  12:type Identity struct {
  20:type MappedPrincipal struct {
  29:type IdentityProvider interface {
  34:type OrganizationMapper interface {

## internal/identity/registry.go
  1:package identity
  14:type ProviderStatus string
  16:const (
  22:type MaterializedProvider struct {
  31:type SettingsReader interface {
  37:type IdPRegistry interface {
  47:type idpRegistry struct {
  65:func NewIdPRegistry(ctx context.Context, configs []OIDCProviderConfig, settings SettingsReader, retryInterval time.Duration, logger *slog.Logger) (IdPRegistry, error) {
  108:func (r *idpRegistry) applyOverlay(ctx context.Context) error {
  135:func (r *idpRegistry) Get(name string) (*MaterializedProvider, bool) {
  146:func (r *idpRegistry) List() []*MaterializedProvider {
  160:func (r *idpRegistry) Mode() domain.AuthMode {
  166:func (r *idpRegistry) Subscribe() <-chan struct{} {
  175:func (r *idpRegistry) Reload(ctx context.Context) error {
  207:func (r *idpRegistry) Run(ctx context.Context) error {
  225:func (r *idpRegistry) retryDegraded(ctx context.Context) {
  273:func (r *idpRegistry) Close() {}
  275:func (r *idpRegistry) broadcast() {
  290:var _ IdPRegistry = (*idpRegistry)(nil)

## internal/integrations/secretbox.go
  1:package integrations
  16:type SecretBox struct {
  24:func NewSecretBox(rawKey []byte) *SecretBox {
  44:func NewSecretBoxFromConfig(configKey string) (*SecretBox, error) {
  58:func (b *SecretBox) Encrypt(plaintext []byte) ([]byte, error) {
  81:func (b *SecretBox) Decrypt(ciphertext []byte) ([]byte, error) {
  107:func (b *SecretBox) EncryptToString(plaintext []byte) (string, error) {
  116:func (b *SecretBox) DecryptFromString(encoded string) ([]byte, error) {

## internal/integrations/service.go
  1:package integrations
  14:type Store = store.Store
  18:type AgentRuntimeResolution struct {
  25:type Service struct {
  37:func NewService(st Store, secret *SecretBox, registry plugins.Registry, resolver *plugins.MCPResolver, logger *slog.Logger) *Service {
  52:func (s *Service) Connect(ctx context.Context, userID, providerID string, fields map[string]string) (domain.IntegrationConnection, error) {
  105:func (s *Service) Disconnect(ctx context.Context, userID, providerID string) error {
  110:func (s *Service) DecryptToken(ctx context.Context, userID, providerID string) (string, error) {
  123:func (s *Service) GetConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  128:func (s *Service) ToggleAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool) (domain.AgentIntegration, error) {
  137:func (s *Service) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  142:func (s *Service) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  148:func (s *Service) BuildSummary(ctx context.Context, userID string) (domain.IntegrationSummary, error) {
  205:func (s *Service) ResolveAgentRuntime(ctx context.Context, userID, agentID string) (AgentRuntimeResolution, error) {
  279:func (s *Service) buildAgentBindings(ctx context.Context, userID, providerID string) []domain.AgentBindingSummary {
  299:func toFormFieldSummaries(fields []plugins.FormField) []domain.FormFieldSummary {
  316:func toMCPServerSummaries(refs []plugins.MCPRef) []domain.MCPServerSummary {

## internal/k8s/clientset.go
  1:package k8s
  15:type Client struct {
  22:func NewClient(kubeconfig string, namespace string) (*Client, error) {
  55:func (c *Client) IntegrationPlugins() dynamic.ResourceInterface {
  60:func (c *Client) MCPServers(namespace string) dynamic.ResourceInterface {
  65:func UnstructuredToIntegrationPlugin(u *unstructured.Unstructured) (*IntegrationPlugin, error) {
  74:func UnstructuredToMCPServer(u *unstructured.Unstructured) (*MCPServer, error) {

## internal/k8s/informers.go
  1:package k8s
  13:type Factory struct {
  20:func NewFactory(client *Client, namespace string, resync time.Duration) *Factory {
  34:func (f *Factory) IntegrationPluginInformer() cache.SharedIndexInformer {
  39:func (f *Factory) MCPServerInformer() cache.SharedIndexInformer {
  44:func (f *Factory) Start(stopCh <-chan struct{}) {
  51:func (f *Factory) WaitForCacheSync(stopCh <-chan struct{}) bool {

## internal/k8s/scheme.go
  1:package k8s
  9:var (
  15:func init() {
  22:func addKnownTypes(scheme *runtime.Scheme) error {

## internal/k8s/types.go
  1:package k8s
  10:const (
  15:var (
  30:type IntegrationPlugin struct {
  37:func (in *IntegrationPlugin) DeepCopyObject() runtime.Object {
  46:func (in *IntegrationPlugin) DeepCopy() *IntegrationPlugin {
  55:func (in *IntegrationPlugin) DeepCopyInto(out *IntegrationPlugin) {
  65:type IntegrationPluginSpec struct {
  77:func (in *IntegrationPluginSpec) DeepCopyInto(out *IntegrationPluginSpec) {
  92:type AuthSpec struct {
  97:func (in *AuthSpec) DeepCopyInto(out *AuthSpec) {
  105:type FormField struct {
  113:type ValidateSpec struct {
  117:func (in *ValidateSpec) DeepCopyInto(out *ValidateSpec) {
  125:type HTTPValidate struct {
  134:func (in *HTTPValidate) DeepCopyInto(out *HTTPValidate) {
  149:type ExtractSpec struct {
  155:type SidecarSpec struct {
  160:type Contributions struct {
  165:func (in *Contributions) DeepCopyInto(out *Contributions) {
  181:type MCPRef struct {
  188:func (in *MCPRef) DeepCopyInto(out *MCPRef) {
  202:type ExternalMCP struct {
  207:type IntegrationPluginStatus struct {
  213:type IntegrationPluginList struct {
  219:func (in *IntegrationPluginList) DeepCopyObject() runtime.Object {
  228:func (in *IntegrationPluginList) DeepCopy() *IntegrationPluginList {
  237:func (in *IntegrationPluginList) DeepCopyInto(out *IntegrationPluginList) {
  250:type MCPServer struct {
  257:func (in *MCPServer) DeepCopyObject() runtime.Object {
  266:func (in *MCPServer) DeepCopy() *MCPServer {
  275:func (in *MCPServer) DeepCopyInto(out *MCPServer) {
  283:type MCPServerSpec struct {
  293:func (in *MCPServerSpec) DeepCopyInto(out *MCPServerSpec) {
  305:type MCPNetworkPolicySpec struct {
  309:func (in *MCPNetworkPolicySpec) DeepCopyInto(out *MCPNetworkPolicySpec) {
  317:type MCPServerStatus struct {
  324:func (in *MCPServerStatus) DeepCopyInto(out *MCPServerStatus) {
  334:type MCPServerList struct {
  340:func (in *MCPServerList) DeepCopyObject() runtime.Object {
  349:func (in *MCPServerList) DeepCopy() *MCPServerList {
  358:func (in *MCPServerList) DeepCopyInto(out *MCPServerList) {

## internal/logging/logging.go
  1:package logging
  8:func New(level string) *slog.Logger {

## internal/plugins/crd_registry.go
  1:package plugins
  18:type CRDRegistry struct {
  33:func NewCRDRegistry(informer cache.SharedIndexInformer, logger *slog.Logger) *CRDRegistry {
  43:func (r *CRDRegistry) Get(id string) (Resolved, bool) {
  51:func (r *CRDRegistry) List() []Resolved {
  63:func (r *CRDRegistry) Subscribe() <-chan struct{} {
  74:func (r *CRDRegistry) Run(ctx context.Context) error {
  102:type syntheticEnvelope struct {
  109:type syntheticMeta struct {
  115:func (r *CRDRegistry) upsert(obj any) {
  161:func (r *CRDRegistry) delete(obj any) {
  189:func (r *CRDRegistry) broadcast() {

## internal/plugins/db_registry.go
  1:package plugins
  18:type ManifestLister interface {
  24:type DBRegistry struct {
  40:func NewDBRegistry(store ManifestLister, interval time.Duration, logger *slog.Logger) *DBRegistry {
  53:func (r *DBRegistry) Get(id string) (Resolved, bool) {
  61:func (r *DBRegistry) List() []Resolved {
  73:func (r *DBRegistry) Subscribe() <-chan struct{} {
  83:func (r *DBRegistry) Run(ctx context.Context) error {
  106:func (r *DBRegistry) poll(ctx context.Context) {
  176:func (r *DBRegistry) broadcast() {

## internal/plugins/file_registry.go
  1:package plugins
  17:type FileRegistry struct {
  34:func NewFileRegistry(dir string, logger *slog.Logger) *FileRegistry {
  45:func (r *FileRegistry) Get(id string) (Resolved, bool) {
  53:func (r *FileRegistry) List() []Resolved {
  65:func (r *FileRegistry) Subscribe() <-chan struct{} {
  77:func (r *FileRegistry) Run(ctx context.Context) error {
  103:func (r *FileRegistry) watch(ctx context.Context, watcher *fsnotify.Watcher) {
  143:func (r *FileRegistry) scan() {
  224:func (r *FileRegistry) broadcast() {

## internal/plugins/manifest.go
  1:package plugins
  14:type Manifest struct {
  21:type Metadata struct {
  26:type ManifestSpec struct {
  38:type AuthSpec struct {
  43:type FormField struct {
  51:type ValidateSpec struct {
  55:type HTTPValidate struct {
  64:type ExtractSpec struct {
  70:type SidecarSpec struct {
  75:type Contributions struct {
  80:type MCPRef struct {
  87:type ExternalMCP struct {
  92:var idRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
  94:func ParseManifest(data []byte) (*Manifest, error) {
  105:func (m *Manifest) Validate() error {
  193:func (m *Manifest) validateTemplates() error {
  236:func sandboxedFuncs() template.FuncMap {
  251:func ParseTimeout(s string) (time.Duration, error) {

## internal/plugins/mcp_resolver.go
  1:package plugins
  17:type ResolvedMCPServer struct {
  26:type MCPResolver struct {
  33:func NewMCPResolver(informer cache.SharedIndexInformer, namespace string, logger *slog.Logger) *MCPResolver {
  43:func (r *MCPResolver) Resolve(ctx context.Context, refs []MCPRef, vars TemplateContext) ([]ResolvedMCPServer, error) {
  57:func (r *MCPResolver) resolveOne(_ context.Context, ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
  67:func (r *MCPResolver) resolveExternal(ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
  94:func (r *MCPResolver) resolveCluster(ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
  132:func (r *MCPResolver) renderEnvFrom(envFrom map[string]string, vars TemplateContext) (map[string]string, error) {

## internal/plugins/registry.go
  1:package plugins
  15:type Resolved struct {
  21:type Registry interface {
  31:type MergedRegistry struct {
  47:func NewMerged(logger *slog.Logger, sources ...Registry) *MergedRegistry {
  57:func (m *MergedRegistry) Get(id string) (Resolved, bool) {
  65:func (m *MergedRegistry) List() []Resolved {
  77:func (m *MergedRegistry) Subscribe() <-chan struct{} {
  87:func (m *MergedRegistry) Run(ctx context.Context) error {
  120:func (m *MergedRegistry) rebuild() {
  191:func (m *MergedRegistry) broadcast() {
  206:func specHash(spec ManifestSpec) string {

## internal/plugins/sidecar.go
  1:package plugins
  12:type SidecarClient struct {
  16:func NewSidecarClient() *SidecarClient {
  20:func (c *SidecarClient) httpClient() *http.Client {
  27:func (c *SidecarClient) Validate(ctx context.Context, m *Manifest, vars TemplateContext) (ValidationResult, error) {
  88:func (c *SidecarClient) BuildEnv(ctx context.Context, m *Manifest, vars TemplateContext) (map[string]string, error) {

## internal/plugins/template.go
  1:package plugins
  15:type TemplateContext struct {
  20:type ValidationResult struct {
  26:type HTTPEvaluator struct {
  30:func NewHTTPEvaluator() *HTTPEvaluator {
  34:func (e *HTTPEvaluator) Validate(ctx context.Context, m *Manifest, vars TemplateContext) (ValidationResult, error) {
  117:func (e *HTTPEvaluator) BuildEnv(m *Manifest, vars TemplateContext) (map[string]string, error) {
  129:func renderTemplate(name, tmpl string, vars TemplateContext) (string, error) {
  141:func jsonPath(root any, path string) (string, error) {

## internal/sandbox/docker_demo.go
  1:package sandbox
  11:type StartRequest struct {
  24:type RuntimeLease struct {
  31:type RuntimeProvider interface {
  36:type CommandRunner interface {
  40:type DockerCLI struct{}
  42:func (DockerCLI) Run(ctx context.Context, args ...string) error {
  46:type DockerDemoProvider struct {
  52:func (p DockerDemoProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
  100:func (p DockerDemoProvider) Stop(ctx context.Context, agentID string) error {
  108:func normalizeRuntime(runtime string) string {
  116:func isKnownRuntime(runtime string) bool {

## internal/sandbox/k8s_resources.go
  1:package sandbox
  12:func BuildWorkspacePVC(agentID, sandboxID, storageClassName, size string) (*corev1.PersistentVolumeClaim, error) {
  38:func BuildRuntimePod(spec AgentPodSpec) *corev1.Pod {
  57:func buildRuntimeContainer(spec ContainerSpec) corev1.Container {
  89:func buildEnvVars(env map[string]string) []corev1.EnvVar {
  105:func buildSecretEnvVars(envFrom []EnvFromSource) []corev1.EnvVar {
  127:func buildVolumeMounts(mounts []VolumeMount) []corev1.VolumeMount {
  135:func buildVolumes(volumes []VolumeSpec) []corev1.Volume {
  158:func boolPtr(v bool) *bool { return &v }

## internal/sandbox/kubernetes_provider.go
  1:package sandbox
  20:type KubernetesRuntimeProviderConfig struct {
  39:type KubernetesRuntimeProvider struct {
  45:func NewKubernetesRuntimeProvider(cfg KubernetesRuntimeProviderConfig) (*KubernetesRuntimeProvider, error) {
  83:func (p *KubernetesRuntimeProvider) Config() KubernetesRuntimeProviderConfig {
  87:func (p *KubernetesRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
  180:const defaultPodReadyTimeout = 120 * time.Second
  183:const podReadinessPollInterval = 500 * time.Millisecond
  189:func (p *KubernetesRuntimeProvider) waitForPodReady(ctx context.Context, podName string) error {
  229:func allContainersReady(pod *corev1.Pod) bool {
  243:func buildPodErrorMessage(pod *corev1.Pod) string {
  276:func (p *KubernetesRuntimeProvider) collectPodFailureInfo(ctx context.Context, podName string, timeout time.Duration) error {
  314:func (p *KubernetesRuntimeProvider) validateSecretKey(ctx context.Context, name, key string) error {
  328:func (p *KubernetesRuntimeProvider) Stop(ctx context.Context, agentID string) error {
  357:type deleteFunc func(context.Context, string, metav1.DeleteOptions) error
  359:func deleteIgnoreNotFound(ctx context.Context, deleteFn deleteFunc, name string) error {
  366:func buildKubernetesClient() (kubernetes.Interface, error) {
  387:func (p *KubernetesRuntimeProvider) namespace() string { return p.cfg.Namespace }
  389:func (p *KubernetesRuntimeProvider) createOrUpdateSecret(ctx context.Context, secret *corev1.Secret) error {
  413:func (p *KubernetesRuntimeProvider) createOrUpdatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
  431:func (p *KubernetesRuntimeProvider) createOrUpdateNetworkPolicy(ctx context.Context, policy *networkingv1.NetworkPolicy) error {
  453:func (p *KubernetesRuntimeProvider) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {

## internal/sandbox/mock_runtime.go
  1:package sandbox
  5:type MockRuntimeProvider struct{}
  7:func (MockRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
  15:func (MockRuntimeProvider) Stop(ctx context.Context, agentID string) error {

## internal/sandbox/networkpolicy.go
  1:package sandbox
  16:type NetworkPolicyMode string
  18:const (
  24:type EgressRule struct {
  31:type NetworkPolicySpec struct {
  39:func NetworkPolicySpecFromConfig(enabled bool, mode string, cidrs string) NetworkPolicySpec {
  56:func RuntimeLabels(agentID, runtime, sandboxID string) map[string]string {
  73:func BuildRuntimeNetworkPolicy(agentID, sandboxID string, spec NetworkPolicySpec) (*networkingv1.NetworkPolicy, error) {
  121:func tcpProtocol() *corev1.Protocol {
  126:func udpProtocol() *corev1.Protocol {
  131:func portOrNil(port int32) *intstr.IntOrString {
  136:func labelValue(value string) string {

## internal/sandbox/podspec.go
  1:package sandbox
  5:type AgentPodRequest struct {
  30:type SecretKeyRef struct {
  36:type AgentPodSpec struct {
  49:type ContainerSpec struct {
  66:type EnvFromSource struct {
  72:type VolumeSpec struct {
  80:type VolumeMount struct {
  86:func BuildAgentPodSpec(req AgentPodRequest) AgentPodSpec {

## internal/sandbox/provider.go
  1:package sandbox
  5:type Provider interface {
  9:type AgentRequest struct {
  21:type KataProviderConfig struct {
  26:type KataProvider struct {
  31:func NewKataProvider(cfg KataProviderConfig) *KataProvider {
  39:func (p *KataProvider) BuildAgentPod(req AgentRequest) (AgentPodSpec, error) {

## internal/sandbox/secrets.go
  1:package sandbox
  11:type SecretRef struct {
  20:type RuntimeSecretStore interface {
  25:type KubernetesSecretStore struct {
  29:func (s KubernetesSecretStore) BuildRuntimeTokenSecret(agentID, token string) (SecretRef, *corev1.Secret, error) {
  58:func (s KubernetesSecretStore) DeleteRuntimeTokenName(agentID string) string {

## internal/security/audit.go
  1:package security
  9:type RiskLevel string
  11:const (
  19:type Decision string
  21:const (
  29:type Finding struct {
  37:type ScanInput struct {
  42:type ScanResult struct {
  48:type DeterministicScanner struct{}
  50:func NewDeterministicScanner() DeterministicScanner { return DeterministicScanner{} }
  52:func (DeterministicScanner) Scan(input ScanInput) ScanResult {
  71:func ContentDigest(text string) string {
  76:func containsAny(text string, needles ...string) bool {
  85:func evidence(text string) string {
  93:func maxRisk(findings []Finding) RiskLevel {
  103:func riskRank(risk RiskLevel) int {

## internal/security/policy.go
  1:package security
  3:type PolicyMode string
  5:const (
  11:type Policy struct {
  16:func DefaultPolicy() Policy {
  20:func EvaluatePolicy(policy Policy, result ScanResult) Decision {
  41:func DecisionAllowsUse(decision Decision) bool {

## internal/store/factory.go
  1:package store
  8:type Config struct {
  13:func Open(cfg Config) (Store, error) {

## internal/store/postgres.go
  1:package store
  17:type Postgres struct{ db *sql.DB }
  19:func NewPostgres(dsn string) (*Postgres, error) {
  37:func runMigrations(ctx context.Context, db *sql.DB) error {
  61:func (p *Postgres) Close() error { return p.db.Close() }
  65:func (p *Postgres) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
  86:func (p *Postgres) GetUser(ctx context.Context, userID string) (domain.User, error) {
  100:func (p *Postgres) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
  114:func (p *Postgres) ListUsers(ctx context.Context) ([]domain.User, error) {
  132:func (p *Postgres) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
  167:func (p *Postgres) GetPasswordHash(ctx context.Context, username string) (string, error) {
  180:func (p *Postgres) SetPasswordHash(ctx context.Context, username, hash string) error {
  188:func (p *Postgres) CreateAgent(ctx context.Context, ownerUserID, name, description, runtime, model, systemPrompt string) (domain.Agent, error) {
  207:func (p *Postgres) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
  221:func (p *Postgres) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
  239:func (p *Postgres) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
  255:func (p *Postgres) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
  271:func (p *Postgres) DeleteAgent(ctx context.Context, agentID string) error {
  288:func (p *Postgres) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
  307:func (p *Postgres) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
  321:func (p *Postgres) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
  339:func (p *Postgres) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
  379:func (p *Postgres) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
  393:func (p *Postgres) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
  411:func (p *Postgres) UpsertIntegrationConnection(ctx context.Context, connection domain.IntegrationConnection) (domain.IntegrationConnection, error) {
  438:func (p *Postgres) GetIntegrationConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  455:func (p *Postgres) ListUserIntegrationProviderIDs(ctx context.Context, userID string) ([]string, error) {
  476:func (p *Postgres) DeleteIntegrationConnection(ctx context.Context, userID, providerID string) error {
  492:func (p *Postgres) UpsertAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool, revision int64, status string) (domain.AgentIntegration, error) {
  509:func (p *Postgres) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  531:func (p *Postgres) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  549:func (p *Postgres) ListPluginManifests(ctx context.Context, enabledOnly bool) ([]domain.PluginManifest, error) {
  571:func (p *Postgres) GetPluginManifest(ctx context.Context, id string) (domain.PluginManifest, error) {
  585:func (p *Postgres) UpsertPluginManifest(ctx context.Context, m domain.PluginManifest) (domain.PluginManifest, error) {
  602:func (p *Postgres) DeletePluginManifest(ctx context.Context, id string) error {
  615:func (p *Postgres) GetUserIDByIdentity(ctx context.Context, providerName, subject string) (string, bool, error) {
  630:func (p *Postgres) TouchIdentityLogin(ctx context.Context, providerName, subject string) error {
  644:func (p *Postgres) CreateUserAndIdentity(ctx context.Context, args CreateUserAndIdentityArgs) (domain.User, error) {
  689:func (p *Postgres) LinkIdentityToUser(ctx context.Context, userID, providerName, subject, email, displayName string) error {
  713:func (p *Postgres) ListUserIdentities(ctx context.Context, userID string) ([]domain.UserIdentity, error) {
  742:func (p *Postgres) UnlinkIdentity(ctx context.Context, userID, providerName, subject string) error {
  758:func (p *Postgres) GetAppSetting(ctx context.Context, key string) (string, error) {
  770:func (p *Postgres) SetAppSetting(ctx context.Context, key, value string) error {
  782:func (p *Postgres) ListAppSettings(ctx context.Context, prefix string) (map[string]string, error) {
  807:func (p *Postgres) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {

## internal/store/store.go
  1:package store
  16:type Store interface {
  77:type CreateUserAndIdentityArgs struct {
  86:var ErrNotFound = errors.New("not found")
  87:var ErrForbidden = errors.New("forbidden")
  88:var ErrConflict = errors.New("conflict")
  89:var ErrInvalidInput = errors.New("invalid input")
  91:type Memory struct {
  111:func NewMemory() *Memory {
  121:func (m *Memory) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
  145:func (m *Memory) GetUser(ctx context.Context, userID string) (domain.User, error) {
  159:func (m *Memory) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
  173:func (m *Memory) ListUsers(ctx context.Context) ([]domain.User, error) {
  184:func (m *Memory) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
  206:func (m *Memory) GetPasswordHash(_ context.Context, username string) (string, error) {
  217:func (m *Memory) SetPasswordHash(_ context.Context, username, hash string) error {
  226:func (m *Memory) CreateAgent(ctx context.Context, ownerUserID, name, description, runtime, model, systemPrompt string) (domain.Agent, error) {
  253:func (m *Memory) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
  267:func (m *Memory) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
  282:func (m *Memory) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
  298:func (m *Memory) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
  314:func (m *Memory) DeleteAgent(ctx context.Context, agentID string) error {
  338:func (m *Memory) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
  361:func (m *Memory) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
  375:func (m *Memory) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
  386:func (m *Memory) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
  412:func (m *Memory) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
  427:func (m *Memory) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
  450:func (m *Memory) UpsertIntegrationConnection(ctx context.Context, connection domain.IntegrationConnection) (domain.IntegrationConnection, error) {
  495:func (m *Memory) GetIntegrationConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  509:func (m *Memory) ListUserIntegrationProviderIDs(ctx context.Context, userID string) ([]string, error) {
  528:func (m *Memory) DeleteIntegrationConnection(ctx context.Context, userID, providerID string) error {
  553:func (m *Memory) UpsertAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool, revision int64, status string) (domain.AgentIntegration, error) {
  591:func (m *Memory) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  609:func (m *Memory) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  625:func (m *Memory) ListPluginManifests(ctx context.Context, enabledOnly bool) ([]domain.PluginManifest, error) {
  641:func (m *Memory) GetPluginManifest(ctx context.Context, id string) (domain.PluginManifest, error) {
  654:func (m *Memory) UpsertPluginManifest(ctx context.Context, manifest domain.PluginManifest) (domain.PluginManifest, error) {
  677:func (m *Memory) DeletePluginManifest(ctx context.Context, id string) error {
  692:func (m *Memory) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {
  730:func newID() (string, error) {
  740:func (m *Memory) GetUserIDByIdentity(ctx context.Context, providerName, subject string) (string, bool, error) {
  754:func (m *Memory) TouchIdentityLogin(ctx context.Context, providerName, subject string) error {
  770:func (m *Memory) CreateUserAndIdentity(ctx context.Context, args CreateUserAndIdentityArgs) (domain.User, error) {
  806:func (m *Memory) LinkIdentityToUser(ctx context.Context, userID, providerName, subject, email, displayName string) error {
  841:func (m *Memory) ListUserIdentities(ctx context.Context, userID string) ([]domain.UserIdentity, error) {
  866:func (m *Memory) UnlinkIdentity(ctx context.Context, userID, providerName, subject string) error {
  884:func (m *Memory) GetAppSetting(ctx context.Context, key string) (string, error) {
  893:func (m *Memory) SetAppSetting(ctx context.Context, key, value string) error {
  903:func (m *Memory) ListAppSettings(ctx context.Context, prefix string) (map[string]string, error) {

## migrations/0001_platform.sql

## migrations/0002_integrations.sql

## migrations/0003_agent_description_prompt.sql

## migrations/0004_plugin_manifests.sql

## migrations/0005_integration_scope.sql

## migrations/0006_app_settings.sql

## migrations/0007_user_identities.sql

## migrations/migrate.go
  1:package migrations
  6:var FS embed.FS

