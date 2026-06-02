### cmd/mock-runtime/main.go
  16:type Envelope struct {
  25:func main() {
  90:func generateResponse(input, flavor string) string {
  108:func splitChunks(text string) []string {
  128:func taskText(payload map[string]any) string {
  139:func env(key, fallback string) string {

### cmd/shclop-runtime/main_test.go
  12:func TestRuntimeTokenFromEnvPrefersFile(t *testing.T) {
  25:func TestRuntimeTokenFromEnvFallsBackToEnv(t *testing.T) {
  33:func TestTaskTextExtractsPayloadText(t *testing.T) {
  40:func TestClawEventToEnvelopeMapsError(t *testing.T) {
  51:func TestClawEventToEnvelopeMapsUnknownTypeToError(t *testing.T) {
  62:func TestIsTerminalEnvelope(t *testing.T) {
  71:func TestClawEventToEnvelopeMapsMissingTerminalToError(t *testing.T) {
  82:type assertError string
  84:func (e assertError) Error() string { return string(e) }

### cmd/shclop-runtime/main.go
  15:func main() {
  100:func adapterForRuntime(runtimeName string) claw.Adapter {
  115:func taskText(payload map[string]any) string {
  120:func clawEventToEnvelope(event claw.Event, task gateway.Envelope, seq int) gateway.Envelope {
  148:func isTerminalEnvelope(eventType string) bool {
  152:func env(key, fallback string) string {
  159:func runtimeTokenFromEnv() string {

### cmd/shclop/main.go
  12:func main() {

### internal/api/demo_flow_test.go
  14:func TestFunctionalDemoRoutesBrowserTaskThroughRuntime(t *testing.T) {

### internal/api/runtime_ws_test.go
  14:func TestRuntimeWebSocketAcceptsRuntimeHello(t *testing.T) {
  62:func TestRuntimeWebSocketRequiresToken(t *testing.T) {

### internal/api/server_test.go
  18:func TestHealthz(t *testing.T) {
  31:func TestReadyz(t *testing.T) {
  44:func TestMetricsEnabled(t *testing.T) {
  59:func TestMetricsDisabled(t *testing.T) {
  71:func TestLoginAndGetMe(t *testing.T) {
  108:func TestBrowserWebSocketRejectsQueryToken(t *testing.T) {
  121:func TestLoginInvalidCredentials(t *testing.T) {
  133:func TestAdminCreateUser(t *testing.T) {
  170:func TestAdminDisableUser(t *testing.T) {
  204:func TestAgentsCreateAndList(t *testing.T) {
  259:func TestAgentRequiresValidRuntime(t *testing.T) {
  272:func TestAdminModels(t *testing.T) {
  330:func TestAdminLLMGateway(t *testing.T) {
  356:func TestAdminOverview(t *testing.T) {
  383:func TestActivity(t *testing.T) {
  409:func TestModels_EnabledModels(t *testing.T) {
  481:func TestModels_GatewayDiscovery_FiltersByGateway(t *testing.T) {
  554:func TestModels_GatewayDiscovery_FailsWith502(t *testing.T) {
  589:func TestModels_GatewayDiscovery_NoGatewayConfig(t *testing.T) {
  630:func TestAgentsRequireAuth(t *testing.T) {
  643:func TestWrongMethods(t *testing.T) {
  672:func TestServesFrontend(t *testing.T) {
  703:func TestRequireBootstrapPassword_ProductionConfigRequiresPassword(t *testing.T) {
  722:func TestRequireBootstrapPassword_DevConfigAllowsDefaultPassword(t *testing.T) {
  736:func TestRequireBootstrapPassword_InmemoryStoreAllowsDefaultPassword(t *testing.T) {
  750:func TestRequireBootstrapPassword_MockProviderAllowsDefaultPassword(t *testing.T) {
  764:func TestRequireBootstrapPassword_EnvVarSetAllowsProduction(t *testing.T) {
  779:func TestStartAgentWithModelRequiresFullyConfiguredGateway(t *testing.T) {
  811:func TestStartAgentWithModelAcceptsInternalGatewayWithoutSecret(t *testing.T) {
  845:func TestStopAgentRevokesRuntimeToken(t *testing.T) {
  889:func TestIntegrations_ListReturnsProviders(t *testing.T) {
  922:func TestIntegrations_ConnectAndDisconnectGitHub(t *testing.T) {
  1022:func TestIntegrations_RequiresAuth(t *testing.T) {
  1044:func TestIntegrations_AgentToggleEnforceOwnership(t *testing.T) {
  1142:func TestIntegrations_AgentToggleRequiresConnection(t *testing.T) {
  1163:func newTestGitHubServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
  1176:func newTestServer() *Server {
  1180:func newTestServerWithConfig(cfg config.Config) *Server {
  1191:func newTestServerWithGitHub(t *testing.T, validateURL string) *Server {
  1213:func githubManifestYAML(validateURL string) string {
  1251:func loginAsAdmin(t *testing.T, server *Server) string {
  1256:func loginAs(t *testing.T, server *Server, username, password string) string {
  1268:func doJSON(t *testing.T, server *Server, method, path string, payload any, token string) *httptest.ResponseRecorder {
  1291:func assertJSONField(t *testing.T, body []byte, key string, want string) string {
  1304:func assertJSONObject(t *testing.T, body []byte, key string) map[string]any {
  1317:func assertJSONArray(t *testing.T, body []byte, key string) []map[string]any {

### internal/api/server.go
  38:var wsUpgrader = websocket.Upgrader{CheckOrigin: sameOriginOrNoOrigin}
  40:func sameOriginOrNoOrigin(r *http.Request) bool {
  52:type Server struct {
  71:type MetricsCollectors struct {
  86:func newMetricsCollectors() *MetricsCollectors {
  152:type activityEntry struct {
  161:func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
  270:func (s *Server) requireBootstrapPassword() error {
  280:func (s *Server) bootstrapAdmin() {
  324:func requestContext() requestCtx {
  328:type requestCtx struct{}
  330:func (requestCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
  331:func (requestCtx) Done() <-chan struct{}       { return nil }
  332:func (requestCtx) Err() error                  { return nil }
  333:func (requestCtx) Value(key any) any           { return nil }
  335:func sandboxProviderFromConfig(cfg config.Config) (sandbox.RuntimeProvider, error) {
  367:func (s *Server) ListenAndServe() error {
  382:func (s *Server) Handler() http.Handler {
  386:func (s *Server) withMetrics(next http.Handler) http.Handler {
  397:type statusWriter struct {
  402:func (sw *statusWriter) WriteHeader(code int) {
  407:func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
  414:func (s *Server) routes() http.Handler {
  463:func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
  471:func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
  485:func (s *Server) handleMetrics() http.Handler {
  496:func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
  535:func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
  549:func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
  560:func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
  597:func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
  658:func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
  674:func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  696:func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  842:func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  880:func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  914:func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
  922:func (s *Server) handleIntegration(w http.ResponseWriter, r *http.Request) {
  947:func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
  961:func (s *Server) handleConnectIntegration(w http.ResponseWriter, r *http.Request, providerID string) {
  1011:func (s *Server) handleDisconnectIntegration(w http.ResponseWriter, r *http.Request, providerID string) {
  1027:func (s *Server) handleAgentIntegration(w http.ResponseWriter, r *http.Request, agentID, providerID string) {
  1094:func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
  1105:func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
  1125:func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
  1175:func (s *Server) handleAdminUser(w http.ResponseWriter, r *http.Request) {
  1189:func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request, targetUserID string) {
  1228:func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
  1237:func (s *Server) handleListEnabledModels(w http.ResponseWriter, r *http.Request) {
  1292:func (s *Server) fetchLiteLLMModels(ctx context.Context, baseURL, apiKey string) (map[string]bool, error) {
  1338:func (s *Server) handleAdminModels(w http.ResponseWriter, r *http.Request) {
  1349:func (s *Server) handleAdminListModels(w http.ResponseWriter, r *http.Request) {
  1369:func (s *Server) handleAdminCreateModel(w http.ResponseWriter, r *http.Request) {
  1402:func (s *Server) handleAdminModel(w http.ResponseWriter, r *http.Request) {
  1416:func (s *Server) handleAdminUpdateModel(w http.ResponseWriter, r *http.Request, modelID string) {
  1449:func (s *Server) handleAdminLLMGateway(w http.ResponseWriter, r *http.Request) {
  1460:func (s *Server) handleAdminGetLLMGateway(w http.ResponseWriter, r *http.Request) {
  1477:func (s *Server) handleAdminUpdateLLMGateway(w http.ResponseWriter, r *http.Request) {
  1507:func (s *Server) handleAdminPlugins(w http.ResponseWriter, r *http.Request) {
  1518:func (s *Server) handleAdminPlugin(w http.ResponseWriter, r *http.Request) {
  1535:func (s *Server) handleAdminListPlugins(w http.ResponseWriter, r *http.Request) {
  1555:func (s *Server) handleAdminUpsertPlugin(w http.ResponseWriter, r *http.Request, pathID string) {
  1614:func (s *Server) handleAdminDeletePlugin(w http.ResponseWriter, r *http.Request, pluginID string) {
  1635:func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
  1671:func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
  1683:func (s *Server) recordActivity(eventType, actorID, agentID, message string, details map[string]any) {
  1696:func (s *Server) activitySnapshot() []activityEntry {
  1702:func (s *Server) activityForUser(user domain.User) []activityEntry {
  1718:func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
  1804:func (s *Server) handleRuntimeWebSocket(w http.ResponseWriter, r *http.Request) {
  1850:func (s *Server) validRuntimeToken(agentID, secret string) bool {
  1858:func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
  1880:func (s *Server) requireUserFromRequest(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
  1903:func (s *Server) enforceCurrentUser(w http.ResponseWriter, r *http.Request, cached domain.User) (domain.User, bool) {
  1916:func chatEventResponse(event gateway.Envelope) map[string]any {
  1931:func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
  1938:func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
  1944:func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
  1972:func writeJSON(w http.ResponseWriter, status int, value any) error {
  1985:func methodNotAllowed(w http.ResponseWriter, allow string) {
  1990:func randomSecret() (string, error) {
  1998:func randomHexID() string {

### internal/auth/auth_test.go
  12:type testStore struct {
  17:func newTestStore() *testStore {
  24:func (s *testStore) addUser(id, username, password, role string) {
  34:func (s *testStore) GetUserByUsername(_ context.Context, username string) (domain.User, error) {
  42:func (s *testStore) GetUser(_ context.Context, userID string) (domain.User, error) {
  51:func (s *testStore) GetPasswordHash(_ context.Context, username string) (string, error) {
  59:func (s *testStore) SetPasswordHash(_ context.Context, username, hash string) error {
  64:var errNotFound = &testError{"not found"}
  66:type testError struct{ msg string }
  68:func (e *testError) Error() string { return e.msg }
  70:func TestLoginSuccess(t *testing.T) {
  96:func TestLoginInvalidPassword(t *testing.T) {
  107:func TestLoginUnknownUser(t *testing.T) {
  117:func TestResolveInvalidToken(t *testing.T) {
  127:func TestLoginDisabledUser(t *testing.T) {
  145:func TestTokenIsolation(t *testing.T) {

### internal/auth/auth.go
  15:type PasswordHasher interface {
  21:type UserStore interface {
  26:type Service struct {
  33:func NewService(store UserStore, hasher PasswordHasher) *Service {
  41:func (s *Service) Login(ctx context.Context, username, password string) (domain.User, string, error) {
  71:func (s *Service) Resolve(token string) (domain.User, bool) {
  78:func tokenID() (string, error) {

### internal/claw/adapter_test.go
  11:func TestDemoAdapterEmitsStructuredEvents(t *testing.T) {
  27:func TestSubprocessAdapterStreamsStdout(t *testing.T) {
  59:func TestSubprocessAdapterStreamsLongOutput(t *testing.T) {
  89:func TestSubprocessAdapterReportsNonZeroExit(t *testing.T) {
  110:func TestSubprocessAdapterClosesOnCanceledContext(t *testing.T) {
  134:func TestSubprocessAdapterClosesWhenGrandchildHoldsPipes(t *testing.T) {

### internal/claw/adapter.go
  5:type Task struct {
  10:type EventType string
  12:const (
  19:type Event struct {
  26:type Adapter interface {
  30:type DemoAdapter struct{ Flavor string }
  32:func (a DemoAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

### internal/claw/nanoclaw.go
  16:type NanoclawAdapter struct{}
  19:type OpenclawAdapter struct{}
  21:func (a NanoclawAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
  36:func (a OpenclawAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
  54:func writeNanoclawConfig() error {
  105:func buildSystemPrompt() string {
  134:func nanoclawConfigDir() string {
  150:func testWritable(dir string) bool {
  164:func nanoclawEnv() []string {

### internal/claw/openai_test.go
  14:func TestOpenAIAdapterReturnsErrorWithoutEnv(t *testing.T) {
  33:func TestOpenAIAdapterReturnsTextResponse(t *testing.T) {
  103:func TestOpenAIAdapterExecutesBashTool(t *testing.T) {
  190:func TestOpenAIAdapterHandlesAPIError(t *testing.T) {
  224:func TestOpenAIAdapterContextCancellation(t *testing.T) {

### internal/claw/openai.go
  21:type OpenAIAdapter struct{}
  23:const systemPrompt = `You are an AI agent running inside a Linux container. You have full shell access via the bash tool.
  37:var bashTool = map[string]any{
  55:type chatMessage struct {
  63:type toolCall struct {
  72:func (a OpenAIAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
  231:func runBash(ctx context.Context, command string) (string, error) {

### internal/claw/subprocess.go
  12:type SubprocessAdapter struct {
  18:func (a SubprocessAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

### internal/config/config_test.go
  5:func TestEnvBool(t *testing.T) {

### internal/config/config.go
  8:type Config struct {
  56:func Default() Config {
  105:func env(key, fallback string) string {
  112:func envBool(key string, fallback bool) bool {

### internal/controllers/mcpserver_controller_test.go
  22:var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
  24:func makeTestMCPServer(name, namespace, image string, port int32, replicas int32) *unstructured.Unstructured {
  51:func newTestSetup(t *testing.T, ns string, objs ...*unstructured.Unstructured) (*k8s.Client, cache.SharedIndexInformer, context.CancelFunc) {
  95:func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
  107:func TestReconcileCreatesDeploymentServiceNetworkPolicy(t *testing.T) {
  196:func TestReconcileIdempotent(t *testing.T) {
  231:func TestReconcileCleanup(t *testing.T) {

### internal/controllers/mcpserver_controller.go
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

### internal/domain/domain.go
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

### internal/gateway/envelope.go
  3:type Envelope struct {

### internal/gateway/mock_runtime_test.go
  13:func TestMockRuntimeStreamsResponse(t *testing.T) {
  27:func TestRegistryDropsEventsFromWrongAgent(t *testing.T) {
  48:func TestRegistryDropsEventsFromStaleConnection(t *testing.T) {
  72:func TestRegistryCancelDoesNotPanicDispatch(t *testing.T) {
  87:func TestRegistryUnregisterCompletesPendingWaiter(t *testing.T) {
  111:func pipeWebSocket(t *testing.T) (*websocket.Conn, *websocket.Conn) {

### internal/gateway/mock_runtime.go
  3:type MockRuntime struct{}
  5:func (MockRuntime) Respond(agentID, sessionID, messageID, text string) []Envelope {

### internal/gateway/registry.go
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

### internal/identity/mockyaml_test.go
  10:func TestMockYAMLProviderAuthenticatesAndMapsPrincipal(t *testing.T) {
  40:func TestMockYAMLProviderRejectsInvalidPassword(t *testing.T) {
  50:func TestMockYAMLProviderRejectsUsersWithoutPassword(t *testing.T) {
  64:func TestStaticOrganizationMapperRejectsMissingTenant(t *testing.T) {
  71:func TestStaticOrganizationMapperRejectsMissingSubject(t *testing.T) {
  78:func writeMockIdentityConfig(t *testing.T) string {
  97:func contains(values []string, target string) bool {

### internal/identity/mockyaml.go
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

### internal/identity/provider.go
  5:type AuthRequest struct {
  12:type Identity struct {
  20:type MappedPrincipal struct {
  29:type IdentityProvider interface {
  34:type OrganizationMapper interface {

### internal/integrations/secretbox_test.go
  8:func TestSecretBoxRoundTrip(t *testing.T) {
  32:func TestSecretBoxDifferentKeys(t *testing.T) {
  48:func TestSecretBoxTamperedCiphertext(t *testing.T) {
  67:func TestSecretBoxDeterministicKeyDerivation(t *testing.T) {
  90:func TestNewSecretBoxFromConfigDevFallback(t *testing.T) {
  110:func TestNewSecretBoxFromConfigWithEnvKey(t *testing.T) {

### internal/integrations/secretbox.go
  16:type SecretBox struct {
  24:func NewSecretBox(rawKey []byte) *SecretBox {
  44:func NewSecretBoxFromConfig(configKey string) (*SecretBox, error) {
  58:func (b *SecretBox) Encrypt(plaintext []byte) ([]byte, error) {
  81:func (b *SecretBox) Decrypt(ciphertext []byte) ([]byte, error) {
  107:func (b *SecretBox) EncryptToString(plaintext []byte) (string, error) {
  116:func (b *SecretBox) DecryptFromString(encoded string) ([]byte, error) {

### internal/integrations/service_test.go
  21:type fakeRegistry struct {
  28:func newFakeRegistry(src string) *fakeRegistry {
  36:func (f *fakeRegistry) set(id string, m plugins.Manifest) {
  46:func (f *fakeRegistry) Get(id string) (plugins.Resolved, bool) {
  56:func (f *fakeRegistry) List() []plugins.Resolved {
  66:func (f *fakeRegistry) Subscribe() <-chan struct{} { return f.notifyCh }
  67:func (f *fakeRegistry) Run(_ context.Context) error { return nil }
  74:func httpTemplateManifest(id, baseURL string) plugins.Manifest {
  110:func httpTemplateMCPManifest(id, baseURL string) plugins.Manifest {
  126:func newTestService(t *testing.T, reg plugins.Registry) (*Service, *store.Memory) {
  139:func fakeGitHubServer(t *testing.T) *httptest.Server {
  162:func TestConnect_HappyPath_HTTPTemplate(t *testing.T) {
  206:func TestConnect_ValidationFailure(t *testing.T) {
  232:func TestConnect_UnregisteredProvider(t *testing.T) {
  246:func TestDisconnect_OrphanedConnection(t *testing.T) {
  274:func TestBuildSummary_OrphanedCredential(t *testing.T) {
  318:func TestToggleAgentIntegration(t *testing.T) {
  347:func TestResolveAgentRuntime_HTTPTemplate_Env(t *testing.T) {
  382:func TestResolveAgentRuntime_ExternalMCP(t *testing.T) {
  421:func TestResolveAgentRuntime_UnregisteredPlugin(t *testing.T) {

### internal/integrations/service.go
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

### internal/k8s/clientset.go
  15:type Client struct {
  22:func NewClient(kubeconfig string, namespace string) (*Client, error) {
  55:func (c *Client) IntegrationPlugins() dynamic.ResourceInterface {
  60:func (c *Client) MCPServers(namespace string) dynamic.ResourceInterface {
  65:func UnstructuredToIntegrationPlugin(u *unstructured.Unstructured) (*IntegrationPlugin, error) {
  74:func UnstructuredToMCPServer(u *unstructured.Unstructured) (*MCPServer, error) {

### internal/k8s/informers.go
  13:type Factory struct {
  20:func NewFactory(client *Client, namespace string, resync time.Duration) *Factory {
  34:func (f *Factory) IntegrationPluginInformer() cache.SharedIndexInformer {
  39:func (f *Factory) MCPServerInformer() cache.SharedIndexInformer {
  44:func (f *Factory) Start(stopCh <-chan struct{}) {
  51:func (f *Factory) WaitForCacheSync(stopCh <-chan struct{}) bool {

### internal/k8s/scheme.go
  9:var (
  15:func init() {
  22:func addKnownTypes(scheme *runtime.Scheme) error {

### internal/k8s/types_test.go
  13:func sampleIntegrationPlugin() *IntegrationPlugin {
  49:func sampleMCPServer() *MCPServer {
  83:func TestIntegrationPluginDeepCopy_DifferentPointer(t *testing.T) {
  95:func TestIntegrationPluginDeepCopy_Isolation(t *testing.T) {
  120:func TestIntegrationPluginDeepCopyObject_Kind(t *testing.T) {
  138:func TestIntegrationPluginListDeepCopy(t *testing.T) {
  157:func TestMCPServerDeepCopy_DifferentPointer(t *testing.T) {
  169:func TestMCPServerDeepCopy_Isolation(t *testing.T) {
  184:func TestMCPServerDeepCopyObject_Kind(t *testing.T) {
  199:func TestMCPServerListDeepCopy(t *testing.T) {
  218:func TestUnstructuredRoundTrip_IntegrationPlugin(t *testing.T) {
  257:func TestUnstructuredRoundTrip_MCPServer(t *testing.T) {

### internal/k8s/types.go
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

### internal/logging/logging.go
  8:func New(level string) *slog.Logger {

### internal/plugins/crd_registry_test.go
  30:func crdTestSetup(t *testing.T, logger *slog.Logger) (
  72:func buildIntegrationPlugin(name, displayName string) *k8s.IntegrationPlugin {
  124:func toUnstructured(t *testing.T, ip *k8s.IntegrationPlugin) *unstructured.Unstructured {
  136:func waitGet(reg *CRDRegistry, id string, timeout time.Duration) (Resolved, bool) {
  148:func waitGone(reg *CRDRegistry, id string, timeout time.Duration) bool {
  160:func drainSubscriber(ch <-chan struct{}) {
  170:func TestCRDRegistryAdd(t *testing.T) {
  198:func TestCRDRegistryUpdate(t *testing.T) {
  238:func TestCRDRegistryDelete(t *testing.T) {
  270:func TestCRDRegistryMalformedSpec(t *testing.T) {
  317:func TestCRDRegistryGoneAfterDeleteConfirmPolling(t *testing.T) {

### internal/plugins/crd_registry.go
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

### internal/plugins/db_registry_test.go
  18:type fakeLister struct {
  24:func newFakeLister(initial ...domain.PluginManifest) *fakeLister {
  28:func (f *fakeLister) ListPluginManifests(_ context.Context, _ bool) ([]domain.PluginManifest, error) {
  40:func (f *fakeLister) set(manifests ...domain.PluginManifest) {
  46:func (f *fakeLister) setErr(err error) {
  56:func dbTestManifest(id, displayName string) string {
  82:const dbTestInterval = 50 * time.Millisecond
  85:func waitDB(ch <-chan struct{}, timeout time.Duration) bool {
  98:func TestDBRegistryInitialPollLoads(t *testing.T) {
  134:func TestDBRegistryParseErrorKeepsPrior(t *testing.T) {
  190:func TestDBRegistryChangeDetection(t *testing.T) {
  227:func TestDBRegistryNoOpOnIdenticalSnapshot(t *testing.T) {
  265:func TestDBRegistryStoreErrorTolerated(t *testing.T) {
  321:func TestDBRegistryEmptyResultClears(t *testing.T) {

### internal/plugins/db_registry.go
  18:type ManifestLister interface {
  24:type DBRegistry struct {
  40:func NewDBRegistry(store ManifestLister, interval time.Duration, logger *slog.Logger) *DBRegistry {
  53:func (r *DBRegistry) Get(id string) (Resolved, bool) {
  61:func (r *DBRegistry) List() []Resolved {
  73:func (r *DBRegistry) Subscribe() <-chan struct{} {
  83:func (r *DBRegistry) Run(ctx context.Context) error {
  106:func (r *DBRegistry) poll(ctx context.Context) {
  176:func (r *DBRegistry) broadcast() {

### internal/plugins/file_registry_test.go
  13:func validManifestYAML(id, displayName string) string {
  40:func writeYAML(t *testing.T, dir, filename, content string) string {
  51:func waitNotify(ch <-chan struct{}, timeout time.Duration) bool {
  60:func newTestLogger() *slog.Logger {
  65:func TestInitialScanLoadsFiles(t *testing.T) {
  92:func TestCreateNewFileFiresNotify(t *testing.T) {
  122:func TestModifyFileFiresNotify(t *testing.T) {
  154:func TestDeleteFileFiresNotify(t *testing.T) {
  183:func TestMalformedFileKeepsPrevious(t *testing.T) {
  221:func TestMissingDirDoesNotError(t *testing.T) {
  237:func TestSkipDotFiles(t *testing.T) {
  261:func TestDebounce(t *testing.T) {

### internal/plugins/file_registry.go
  17:type FileRegistry struct {
  34:func NewFileRegistry(dir string, logger *slog.Logger) *FileRegistry {
  45:func (r *FileRegistry) Get(id string) (Resolved, bool) {
  53:func (r *FileRegistry) List() []Resolved {
  65:func (r *FileRegistry) Subscribe() <-chan struct{} {
  77:func (r *FileRegistry) Run(ctx context.Context) error {
  103:func (r *FileRegistry) watch(ctx context.Context, watcher *fsnotify.Watcher) {
  143:func (r *FileRegistry) scan() {
  224:func (r *FileRegistry) broadcast() {

### internal/plugins/manifest_test.go
  8:const githubManifestYAML = `
  37:const sidecarManifestYAML = `
  59:func TestParseManifest_GitHub(t *testing.T) {
  123:func TestParseManifest_Sidecar(t *testing.T) {
  142:func TestParseManifest_ScopesDefaulting(t *testing.T) {
  171:type rejectCase struct {
  177:func TestParseManifest_Reject(t *testing.T) {
  394:func TestParseManifest_SandboxRejectsExec(t *testing.T) {
  421:func TestParseManifest_SandboxRejectsEnv(t *testing.T) {
  450:func TestParseTimeout(t *testing.T) {

### internal/plugins/manifest.go
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

### internal/plugins/mcp_resolver_test.go
  20:var resolverLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
  22:func makeMCPServerUnstructured(name, namespace, image string, port int64, serviceURL string) *unstructured.Unstructured {
  40:func newResolverInformer(t *testing.T, ns string, objs ...*unstructured.Unstructured) (cache.SharedIndexInformer, context.CancelFunc) {
  67:func TestResolveExternalMCP(t *testing.T) {
  107:func TestResolveClusterRefPresent(t *testing.T) {
  134:func TestResolveClusterRefAbsent(t *testing.T) {
  153:func TestResolveClusterRefDerivedURL(t *testing.T) {
  184:var _ = metav1.GetOptions{}

### internal/plugins/mcp_resolver.go
  17:type ResolvedMCPServer struct {
  26:type MCPResolver struct {
  33:func NewMCPResolver(informer cache.SharedIndexInformer, namespace string, logger *slog.Logger) *MCPResolver {
  43:func (r *MCPResolver) Resolve(ctx context.Context, refs []MCPRef, vars TemplateContext) ([]ResolvedMCPServer, error) {
  57:func (r *MCPResolver) resolveOne(_ context.Context, ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
  67:func (r *MCPResolver) resolveExternal(ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
  94:func (r *MCPResolver) resolveCluster(ref MCPRef, vars TemplateContext) (ResolvedMCPServer, error) {
  132:func (r *MCPResolver) renderEnvFrom(envFrom map[string]string, vars TemplateContext) (map[string]string, error) {

### internal/plugins/registry_test.go
  14:type fakeRegistry struct {
  21:func newFake(src string) *fakeRegistry {
  29:func (f *fakeRegistry) Set(id string, m Manifest) {
  39:func (f *fakeRegistry) Remove(id string) {
  49:func (f *fakeRegistry) Get(id string) (Resolved, bool) {
  59:func (f *fakeRegistry) List() []Resolved {
  69:func (f *fakeRegistry) Subscribe() <-chan struct{} {
  73:func (f *fakeRegistry) Run(_ context.Context) error { return nil }
  76:func minimalManifest(id, displayName string) Manifest {
  105:func TestPrecedence(t *testing.T) {
  142:func TestFallThroughOnRemoval(t *testing.T) {
  186:func TestSubscribeReceivesOnChange(t *testing.T) {
  212:func TestSubscribeNonBlocking(t *testing.T) {
  245:func TestShadowLog(t *testing.T) {
  308:func TestRunIdempotent(t *testing.T) {

### internal/plugins/registry.go
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

### internal/plugins/sidecar_test.go
  14:func sidecarManifest(url, timeout string) *Manifest {
  35:func TestSidecarClient_Validate_HappyPath(t *testing.T) {
  91:func TestSidecarClient_Validate_StatusError(t *testing.T) {
  114:func TestSidecarClient_Validate_Non2xx(t *testing.T) {
  136:func TestSidecarClient_BuildEnv_HappyPath(t *testing.T) {
  162:func TestSidecarClient_Validate_Timeout(t *testing.T) {
  182:func TestSidecarClient_Validate_NetworkUnreachable(t *testing.T) {
  200:func TestSidecarClient_Validate_WrongKind(t *testing.T) {

### internal/plugins/sidecar.go
  12:type SidecarClient struct {
  16:func NewSidecarClient() *SidecarClient {
  20:func (c *SidecarClient) httpClient() *http.Client {
  27:func (c *SidecarClient) Validate(ctx context.Context, m *Manifest, vars TemplateContext) (ValidationResult, error) {
  88:func (c *SidecarClient) BuildEnv(ctx context.Context, m *Manifest, vars TemplateContext) (map[string]string, error) {

### internal/plugins/template_test.go
  12:func makeHTTPManifest(t *testing.T, method, urlTemplate string, extraHeaders map[string]string, extract map[string]string) *Manifest {
  46:func TestValidate_HappyPath(t *testing.T) {
  88:func TestValidate_NonSuccessStatus(t *testing.T) {
  107:func TestValidate_BadJSON(t *testing.T) {
  126:func TestValidate_ContextCancellation(t *testing.T) {
  157:func TestValidate_TemplateRenderingInURL(t *testing.T) {
  183:func TestBuildEnv(t *testing.T) {

### internal/plugins/template.go
  15:type TemplateContext struct {
  20:type ValidationResult struct {
  26:type HTTPEvaluator struct {
  30:func NewHTTPEvaluator() *HTTPEvaluator {
  34:func (e *HTTPEvaluator) Validate(ctx context.Context, m *Manifest, vars TemplateContext) (ValidationResult, error) {
  117:func (e *HTTPEvaluator) BuildEnv(m *Manifest, vars TemplateContext) (map[string]string, error) {
  129:func renderTemplate(name, tmpl string, vars TemplateContext) (string, error) {
  141:func jsonPath(root any, path string) (string, error) {

### internal/sandbox/docker_demo_test.go
  9:func TestDockerDemoProviderBuildsLocalRuntimeCommand(t *testing.T) {
  39:func TestDockerDemoProviderIncludesIntegrationEnv(t *testing.T) {
  72:func TestDockerDemoProviderRejectsUnknownRuntime(t *testing.T) {
  79:type recordingRunner struct{ args []string }
  81:func (r *recordingRunner) Run(ctx context.Context, args ...string) error {

### internal/sandbox/docker_demo.go
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

### internal/sandbox/k8s_resources_test.go
  5:func TestBuildWorkspacePVCDefaultsAndLabels(t *testing.T) {
  21:func TestBuildWorkspacePVCRejectsInvalidSize(t *testing.T) {
  27:func TestBuildRuntimePodHardeningAndVolumes(t *testing.T) {

### internal/sandbox/k8s_resources.go
  12:func BuildWorkspacePVC(agentID, sandboxID, storageClassName, size string) (*corev1.PersistentVolumeClaim, error) {
  38:func BuildRuntimePod(spec AgentPodSpec) *corev1.Pod {
  57:func buildRuntimeContainer(spec ContainerSpec) corev1.Container {
  89:func buildEnvVars(env map[string]string) []corev1.EnvVar {
  105:func buildSecretEnvVars(envFrom []EnvFromSource) []corev1.EnvVar {
  127:func buildVolumeMounts(mounts []VolumeMount) []corev1.VolumeMount {
  135:func buildVolumes(volumes []VolumeSpec) []corev1.Volume {
  158:func boolPtr(v bool) *bool { return &v }

### internal/sandbox/kubernetes_provider_test.go
  21:func seedRunningPodReactor(client *fake.Clientset) {
  40:func TestKubernetesRuntimeProviderStartCreatesSandboxResources(t *testing.T) {
  115:func TestKubernetesRuntimeProviderStartSanitizesLabelsForUnsafeAgentID(t *testing.T) {
  148:func TestKubernetesRuntimeProviderStartDoesNotReplaceExistingPodOrPVCSpec(t *testing.T) {
  168:func TestNewKubernetesRuntimeProviderRejectsEmptyRuntimeClassName(t *testing.T) {
  183:func TestKubernetesRuntimeProviderStartValidatesLLMGatewaySecret(t *testing.T) {
  203:func TestKubernetesRuntimeProviderStartValidatesInputs(t *testing.T) {
  217:func TestKubernetesRuntimeProviderStopDeletesResources(t *testing.T) {
  247:func TestKubernetesRuntimeProviderStopRetainsPVC(t *testing.T) {
  258:func TestKubernetesRuntimeProviderStopReturnsDeleteErrors(t *testing.T) {
  271:func TestKubernetesRuntimeProviderStartWaitsForPodReady(t *testing.T) {
  320:func TestKubernetesRuntimeProviderStartFailsOnPodReadyTimeout(t *testing.T) {
  377:func TestKubernetesRuntimeProviderStartFailsOnPodFailedPhase(t *testing.T) {
  431:func strPtr(s string) *string { return &s }

### internal/sandbox/kubernetes_provider.go
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

### internal/sandbox/mock_runtime.go
  5:type MockRuntimeProvider struct{}
  7:func (MockRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
  15:func (MockRuntimeProvider) Stop(ctx context.Context, agentID string) error {

### internal/sandbox/networkpolicy_test.go
  10:func TestNetworkPolicySpecFromConfigParsesCIDRs(t *testing.T) {
  26:func TestBuildRuntimeNetworkPolicyRestricted(t *testing.T) {
  56:func TestBuildRuntimeNetworkPolicyDisabled(t *testing.T) {
  66:func TestBuildRuntimeNetworkPolicyNoCIDRsKeepsEgressEmpty(t *testing.T) {
  79:func TestBuildRuntimeNetworkPolicyAllowsBackendAndVault(t *testing.T) {
  101:func TestBuildRuntimeNetworkPolicyAllowsDNSAndLLMGateway(t *testing.T) {
  118:func hasPort(rules []networkingv1.NetworkPolicyEgressRule, port int32, protocol corev1.Protocol) bool {

### internal/sandbox/networkpolicy.go
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

### internal/sandbox/podspec_test.go
  8:func TestAgentPodSpecUsesKataAndHardening(t *testing.T) {
  81:func TestAgentPodSpecSecretMountDirUsesFilePath(t *testing.T) {
  94:func TestRuntimeLabelsIncludeExtractionFields(t *testing.T) {
  104:func TestRuntimeLabelsSanitizeValues(t *testing.T) {
  118:func containsAny(s, chars string) bool {
  127:func TestAgentPodSpecIncludesIntegrationEnv(t *testing.T) {
  145:func TestBuildWorkspacePVC(t *testing.T) {
  164:func TestBuildRuntimePod(t *testing.T) {

### internal/sandbox/podspec.go
  5:type AgentPodRequest struct {
  30:type SecretKeyRef struct {
  36:type AgentPodSpec struct {
  49:type ContainerSpec struct {
  66:type EnvFromSource struct {
  72:type VolumeSpec struct {
  80:type VolumeMount struct {
  86:func BuildAgentPodSpec(req AgentPodRequest) AgentPodSpec {

### internal/sandbox/provider_test.go
  5:func TestKataProviderBuildsAgentPodFromRuntimeImageCatalog(t *testing.T) {
  36:func TestKataProviderRejectsUnknownRuntime(t *testing.T) {

### internal/sandbox/provider.go
  5:type Provider interface {
  9:type AgentRequest struct {
  21:type KataProviderConfig struct {
  26:type KataProvider struct {
  31:func NewKataProvider(cfg KataProviderConfig) *KataProvider {
  39:func (p *KataProvider) BuildAgentPod(req AgentRequest) (AgentPodSpec, error) {

### internal/sandbox/secrets_test.go
  5:func TestKubernetesSecretStoreBuildsSecretRef(t *testing.T) {
  19:func TestKubernetesSecretStoreValidatesInputs(t *testing.T) {

### internal/sandbox/secrets.go
  11:type SecretRef struct {
  20:type RuntimeSecretStore interface {
  25:type KubernetesSecretStore struct {
  29:func (s KubernetesSecretStore) BuildRuntimeTokenSecret(agentID, token string) (SecretRef, *corev1.Secret, error) {
  58:func (s KubernetesSecretStore) DeleteRuntimeTokenName(agentID string) string {

### internal/security/audit_test.go
  5:func TestScannerApprovesBenignContent(t *testing.T) {
  16:func TestScannerRejectsSecretExfiltration(t *testing.T) {
  30:func TestContentDigestStable(t *testing.T) {

### internal/security/audit.go
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

### internal/security/policy_test.go
  5:func TestPolicyEnforceRejectsHighRisk(t *testing.T) {
  12:func TestPolicyWarnAllowsWithWarnings(t *testing.T) {
  19:func TestPolicyEnforceApprovesLowRiskWithWarnings(t *testing.T) {

### internal/security/policy.go
  3:type PolicyMode string
  5:const (
  11:type Policy struct {
  16:func DefaultPolicy() Policy {
  20:func EvaluatePolicy(policy Policy, result ScanResult) Decision {
  41:func DecisionAllowsUse(decision Decision) bool {

### internal/store/factory_test.go
  5:func TestOpenRejectsPostgresWithoutDSN(t *testing.T) {
  12:func TestOpenCreatesMemoryStoreByDefault(t *testing.T) {

### internal/store/factory.go
  8:type Config struct {
  13:func Open(cfg Config) (Store, error) {

### internal/store/postgres.go
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
  615:func (p *Postgres) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {

### internal/store/store_test.go
  11:func TestMemoryStore_IntegrationConnection(t *testing.T) {
  77:func TestMemoryStore_AgentIntegration(t *testing.T) {
  136:func TestMemoryStore_PluginManifestCRUD(t *testing.T) {
  237:func TestMemoryStore_IntegrationConnection_UpsertRevisionAutoIncrement(t *testing.T) {

### internal/store/store.go
  14:type Store interface {
  62:var ErrNotFound = errors.New("not found")
  63:var ErrForbidden = errors.New("forbidden")
  64:var ErrConflict = errors.New("conflict")
  65:var ErrInvalidInput = errors.New("invalid input")
  67:type Memory struct {
  84:func NewMemory() *Memory {
  93:func (m *Memory) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
  117:func (m *Memory) GetUser(ctx context.Context, userID string) (domain.User, error) {
  131:func (m *Memory) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
  145:func (m *Memory) ListUsers(ctx context.Context) ([]domain.User, error) {
  156:func (m *Memory) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
  178:func (m *Memory) GetPasswordHash(_ context.Context, username string) (string, error) {
  189:func (m *Memory) SetPasswordHash(_ context.Context, username, hash string) error {
  198:func (m *Memory) CreateAgent(ctx context.Context, ownerUserID, name, description, runtime, model, systemPrompt string) (domain.Agent, error) {
  225:func (m *Memory) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
  239:func (m *Memory) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
  254:func (m *Memory) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
  270:func (m *Memory) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
  286:func (m *Memory) DeleteAgent(ctx context.Context, agentID string) error {
  310:func (m *Memory) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
  333:func (m *Memory) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
  347:func (m *Memory) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
  358:func (m *Memory) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
  384:func (m *Memory) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
  399:func (m *Memory) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
  422:func (m *Memory) UpsertIntegrationConnection(ctx context.Context, connection domain.IntegrationConnection) (domain.IntegrationConnection, error) {
  467:func (m *Memory) GetIntegrationConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  481:func (m *Memory) ListUserIntegrationProviderIDs(ctx context.Context, userID string) ([]string, error) {
  500:func (m *Memory) DeleteIntegrationConnection(ctx context.Context, userID, providerID string) error {
  525:func (m *Memory) UpsertAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool, revision int64, status string) (domain.AgentIntegration, error) {
  563:func (m *Memory) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  581:func (m *Memory) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  597:func (m *Memory) ListPluginManifests(ctx context.Context, enabledOnly bool) ([]domain.PluginManifest, error) {
  613:func (m *Memory) GetPluginManifest(ctx context.Context, id string) (domain.PluginManifest, error) {
  626:func (m *Memory) UpsertPluginManifest(ctx context.Context, manifest domain.PluginManifest) (domain.PluginManifest, error) {
  649:func (m *Memory) DeletePluginManifest(ctx context.Context, id string) error {
  664:func (m *Memory) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {
  702:func newID() (string, error) {

### migrations/migrate.go
  6:var FS embed.FS

