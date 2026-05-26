### internal/api/demo_flow_test.go
  14:func TestFunctionalDemoRoutesBrowserTaskThroughRuntime(t *testing.T) {

### internal/api/runtime_ws_test.go
  14:func TestRuntimeWebSocketAcceptsRuntimeHello(t *testing.T) {
  62:func TestRuntimeWebSocketRequiresToken(t *testing.T) {

### internal/api/server_test.go
  19:func TestHealthz(t *testing.T) {
  32:func TestReadyz(t *testing.T) {
  45:func TestMetricsEnabled(t *testing.T) {
  60:func TestMetricsDisabled(t *testing.T) {
  72:func TestLoginAndGetMe(t *testing.T) {
  109:func TestBrowserWebSocketRejectsQueryToken(t *testing.T) {
  122:func TestLoginInvalidCredentials(t *testing.T) {
  134:func TestAdminCreateUser(t *testing.T) {
  171:func TestAdminDisableUser(t *testing.T) {
  205:func TestAgentsCreateAndList(t *testing.T) {
  260:func TestAgentRequiresValidRuntime(t *testing.T) {
  273:func TestAdminModels(t *testing.T) {
  331:func TestAdminLLMGateway(t *testing.T) {
  357:func TestAdminOverview(t *testing.T) {
  384:func TestActivity(t *testing.T) {
  410:func TestModels_EnabledModels(t *testing.T) {
  482:func TestModels_GatewayDiscovery_FiltersByGateway(t *testing.T) {
  555:func TestModels_GatewayDiscovery_FailsWith502(t *testing.T) {
  590:func TestModels_GatewayDiscovery_NoGatewayConfig(t *testing.T) {
  631:func TestAgentsRequireAuth(t *testing.T) {
  644:func TestWrongMethods(t *testing.T) {
  673:func TestServesFrontend(t *testing.T) {
  704:func TestRequireBootstrapPassword_ProductionConfigRequiresPassword(t *testing.T) {
  723:func TestRequireBootstrapPassword_DevConfigAllowsDefaultPassword(t *testing.T) {
  737:func TestRequireBootstrapPassword_InmemoryStoreAllowsDefaultPassword(t *testing.T) {
  751:func TestRequireBootstrapPassword_MockProviderAllowsDefaultPassword(t *testing.T) {
  765:func TestRequireBootstrapPassword_EnvVarSetAllowsProduction(t *testing.T) {
  780:func TestStartAgentWithModelRequiresFullyConfiguredGateway(t *testing.T) {
  812:func TestStartAgentWithModelAcceptsInternalGatewayWithoutSecret(t *testing.T) {
  846:func TestStopAgentRevokesRuntimeToken(t *testing.T) {
  890:func TestIntegrations_ListReturnsProviders(t *testing.T) {
  922:func TestIntegrations_ConnectAndDisconnectGitHub(t *testing.T) {
  1025:func TestIntegrations_RequiresAuth(t *testing.T) {
  1047:func TestIntegrations_AgentToggleEnforceOwnership(t *testing.T) {
  1145:func TestIntegrations_AgentToggleRequiresConnection(t *testing.T) {
  1166:func newTestGitHubServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
  1177:func newTestGitHubProvider(t *testing.T, baseURL string) *integrations.GitHubProvider {
  1187:func newTestServer() *Server {
  1191:func newTestServerWithConfig(cfg config.Config) *Server {
  1199:func loginAsAdmin(t *testing.T, server *Server) string {
  1204:func loginAs(t *testing.T, server *Server, username, password string) string {
  1216:func doJSON(t *testing.T, server *Server, method, path string, payload any, token string) *httptest.ResponseRecorder {
  1239:func assertJSONField(t *testing.T, body []byte, key string, want string) string {
  1252:func assertJSONObject(t *testing.T, body []byte, key string) map[string]any {
  1265:func assertJSONArray(t *testing.T, body []byte, key string) []map[string]any {

### internal/api/server.go
  35:var wsUpgrader = websocket.Upgrader{CheckOrigin: sameOriginOrNoOrigin}
  37:func sameOriginOrNoOrigin(r *http.Request) bool {
  49:type Server struct {
  66:type MetricsCollectors struct {
  81:func newMetricsCollectors() *MetricsCollectors {
  147:type activityEntry struct {
  156:func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
  218:func (s *Server) requireBootstrapPassword() error {
  228:func (s *Server) bootstrapAdmin() {
  272:func requestContext() requestCtx {
  276:type requestCtx struct{}
  278:func (requestCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
  279:func (requestCtx) Done() <-chan struct{}       { return nil }
  280:func (requestCtx) Err() error                  { return nil }
  281:func (requestCtx) Value(key any) any           { return nil }
  283:func sandboxProviderFromConfig(cfg config.Config) (sandbox.RuntimeProvider, error) {
  315:func (s *Server) ListenAndServe() error {
  330:func (s *Server) Handler() http.Handler {
  334:func (s *Server) withMetrics(next http.Handler) http.Handler {
  345:type statusWriter struct {
  350:func (sw *statusWriter) WriteHeader(code int) {
  355:func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
  362:func (s *Server) routes() http.Handler {
  409:func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
  417:func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
  431:func (s *Server) handleMetrics() http.Handler {
  442:func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
  481:func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
  495:func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
  506:func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
  540:func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
  601:func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
  617:func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  639:func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  798:func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request, agentID string) {
  838:func (s *Server) handleIntegrations(w http.ResponseWriter, r *http.Request) {
  846:func (s *Server) handleIntegration(w http.ResponseWriter, r *http.Request) {
  871:func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
  885:func (s *Server) handleConnectIntegration(w http.ResponseWriter, r *http.Request, providerID string) {
  923:func (s *Server) handleDisconnectIntegration(w http.ResponseWriter, r *http.Request, providerID string) {
  939:func (s *Server) handleAgentIntegration(w http.ResponseWriter, r *http.Request, agentID, providerID string) {
  1006:func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
  1017:func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
  1037:func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
  1087:func (s *Server) handleAdminUser(w http.ResponseWriter, r *http.Request) {
  1101:func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request, targetUserID string) {
  1140:func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
  1149:func (s *Server) handleListEnabledModels(w http.ResponseWriter, r *http.Request) {
  1204:func (s *Server) fetchLiteLLMModels(ctx context.Context, baseURL, apiKey string) (map[string]bool, error) {
  1250:func (s *Server) handleAdminModels(w http.ResponseWriter, r *http.Request) {
  1261:func (s *Server) handleAdminListModels(w http.ResponseWriter, r *http.Request) {
  1281:func (s *Server) handleAdminCreateModel(w http.ResponseWriter, r *http.Request) {
  1314:func (s *Server) handleAdminModel(w http.ResponseWriter, r *http.Request) {
  1328:func (s *Server) handleAdminUpdateModel(w http.ResponseWriter, r *http.Request, modelID string) {
  1361:func (s *Server) handleAdminLLMGateway(w http.ResponseWriter, r *http.Request) {
  1372:func (s *Server) handleAdminGetLLMGateway(w http.ResponseWriter, r *http.Request) {
  1389:func (s *Server) handleAdminUpdateLLMGateway(w http.ResponseWriter, r *http.Request) {
  1419:func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
  1455:func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
  1467:func (s *Server) recordActivity(eventType, actorID, agentID, message string, details map[string]any) {
  1480:func (s *Server) activitySnapshot() []activityEntry {
  1486:func (s *Server) activityForUser(user domain.User) []activityEntry {
  1502:func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
  1588:func (s *Server) handleRuntimeWebSocket(w http.ResponseWriter, r *http.Request) {
  1634:func (s *Server) validRuntimeToken(agentID, secret string) bool {
  1642:func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
  1664:func (s *Server) requireUserFromRequest(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
  1687:func (s *Server) enforceCurrentUser(w http.ResponseWriter, r *http.Request, cached domain.User) (domain.User, bool) {
  1700:func chatEventResponse(event gateway.Envelope) map[string]any {
  1715:func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
  1722:func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
  1728:func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
  1756:func writeJSON(w http.ResponseWriter, status int, value any) error {
  1769:func methodNotAllowed(w http.ResponseWriter, allow string) {
  1774:func randomSecret() (string, error) {
  1782:func randomHexID() string {

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
  5:type Task struct{ Text string }
  7:type EventType string
  9:const (
  16:type Event struct {
  23:type Adapter interface {
  27:type DemoAdapter struct{ Flavor string }
  29:func (a DemoAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

### internal/claw/openai_test.go
  13:func TestOpenAIAdapterReturnsErrorWithoutEnv(t *testing.T) {
  32:func TestOpenAIAdapterStreamsSSEResponse(t *testing.T) {
  97:func TestOpenAIAdapterHandlesAPIError(t *testing.T) {
  131:func TestOpenAIAdapterContextCancellation(t *testing.T) {

### internal/claw/openai.go
  21:type OpenAIAdapter struct{}
  23:func (a OpenAIAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

### internal/claw/subprocess.go
  12:type SubprocessAdapter struct {
  18:func (a SubprocessAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {

### internal/config/config_test.go
  5:func TestEnvBool(t *testing.T) {

### internal/config/config.go
  8:type Config struct {
  51:func Default() Config {
  95:func env(key, fallback string) string {
  102:func envBool(key string, fallback bool) bool {

### internal/domain/domain.go
  7:type IntegrationConnection struct {
  21:type AgentIntegration struct {
  33:type IntegrationSummary struct {
  39:type ProviderSummary struct {
  48:type ConnectionMetadata struct {
  57:type AgentBindingSummary struct {
  64:type User struct {
  73:type Agent struct {
  85:type LLMModel struct {
  94:type LLMGatewaySettings struct {
  102:type AdminOverview struct {
  108:type AdminRuntimeConfig struct {
  115:type AdminObservability struct {
  121:type AdminHealthStatus struct {
  127:type CreateAgentInput struct {
  133:type Message struct {

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

### internal/integrations/github_test.go
  10:func TestGitHubProviderValidatePAT_Success(t *testing.T) {
  63:func TestGitHubProviderValidatePAT_NonOKStatus(t *testing.T) {
  81:func TestGitHubProviderValidatePAT_NetworkError(t *testing.T) {
  108:func TestGitHubProviderBuildRuntimeEnv(t *testing.T) {
  119:func TestGitHubProviderValidatePAT_DefaultBaseURL(t *testing.T) {

### internal/integrations/github.go
  10:const gitHubDefaultBaseURL = "https://api.github.com"
  14:type GitHubProvider struct {
  21:type GitHubProviderConfig struct {
  29:func NewGitHubProvider(config GitHubProviderConfig) *GitHubProvider {
  48:func (p *GitHubProvider) ValidatePAT(ctx context.Context, token string) (ValidationResult, error) {
  87:func (p *GitHubProvider) BuildRuntimeEnv(token string) map[string]string {
  94:func (p *GitHubProvider) ProviderID() string {

### internal/integrations/provider.go
  8:type ValidationResult struct {
  17:type Provider interface {

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

### internal/integrations/service.go
  15:type Service struct {
  25:func NewService(store store.Store, secret *SecretBox, logger *slog.Logger) *Service {
  36:func (s *Service) RegisterProvider(p Provider) {
  43:func (s *Service) Provider(providerID string) Provider {
  50:func (s *Service) KnownProviders() []string {
  62:func (s *Service) Connect(ctx context.Context, userID, providerID, token string) (domain.IntegrationConnection, error) {
  107:func (s *Service) Disconnect(ctx context.Context, userID, providerID string) error {
  112:func (s *Service) DecryptToken(ctx context.Context, userID, providerID string) (string, error) {
  125:func (s *Service) GetConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  131:func (s *Service) ToggleAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool) (domain.AgentIntegration, error) {
  140:func (s *Service) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  145:func (s *Service) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  151:func (s *Service) BuildSummary(ctx context.Context, userID string) (domain.IntegrationSummary, error) {
  201:func providerDisplayName(pid string) string {

### internal/logging/logging.go
  8:func New(level string) *slog.Logger {

### internal/sandbox/docker_demo_test.go
  9:func TestDockerDemoProviderBuildsLocalRuntimeCommand(t *testing.T) {
  39:func TestDockerDemoProviderIncludesIntegrationEnv(t *testing.T) {
  72:func TestDockerDemoProviderRejectsUnknownRuntime(t *testing.T) {
  79:type recordingRunner struct{ args []string }
  81:func (r *recordingRunner) Run(ctx context.Context, args ...string) error {

### internal/sandbox/docker_demo.go
  11:type StartRequest struct {
  23:type RuntimeLease struct {
  30:type RuntimeProvider interface {
  35:type CommandRunner interface {
  39:type DockerCLI struct{}
  41:func (DockerCLI) Run(ctx context.Context, args ...string) error {
  45:type DockerDemoProvider struct {
  51:func (p DockerDemoProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
  96:func (p DockerDemoProvider) Stop(ctx context.Context, agentID string) error {
  104:func normalizeRuntime(runtime string) string {
  112:func isKnownRuntime(runtime string) bool {

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
  179:const defaultPodReadyTimeout = 120 * time.Second
  182:const podReadinessPollInterval = 500 * time.Millisecond
  188:func (p *KubernetesRuntimeProvider) waitForPodReady(ctx context.Context, podName string) error {
  228:func allContainersReady(pod *corev1.Pod) bool {
  242:func buildPodErrorMessage(pod *corev1.Pod) string {
  275:func (p *KubernetesRuntimeProvider) collectPodFailureInfo(ctx context.Context, podName string, timeout time.Duration) error {
  313:func (p *KubernetesRuntimeProvider) validateSecretKey(ctx context.Context, name, key string) error {
  327:func (p *KubernetesRuntimeProvider) Stop(ctx context.Context, agentID string) error {
  356:type deleteFunc func(context.Context, string, metav1.DeleteOptions) error
  358:func deleteIgnoreNotFound(ctx context.Context, deleteFn deleteFunc, name string) error {
  365:func buildKubernetesClient() (kubernetes.Interface, error) {
  386:func (p *KubernetesRuntimeProvider) namespace() string { return p.cfg.Namespace }
  388:func (p *KubernetesRuntimeProvider) createOrUpdateSecret(ctx context.Context, secret *corev1.Secret) error {
  412:func (p *KubernetesRuntimeProvider) createOrUpdatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
  430:func (p *KubernetesRuntimeProvider) createOrUpdateNetworkPolicy(ctx context.Context, policy *networkingv1.NetworkPolicy) error {
  452:func (p *KubernetesRuntimeProvider) createOrUpdatePod(ctx context.Context, pod *corev1.Pod) error {

### internal/sandbox/mock_runtime.go
  5:type MockRuntimeProvider struct{}
  7:func (MockRuntimeProvider) Start(ctx context.Context, request StartRequest) (RuntimeLease, error) {
  15:func (MockRuntimeProvider) Stop(ctx context.Context, agentID string) error {

### internal/sandbox/networkpolicy_test.go
  7:func TestNetworkPolicySpecFromConfigParsesCIDRs(t *testing.T) {
  23:func TestBuildRuntimeNetworkPolicyRestricted(t *testing.T) {
  53:func TestBuildRuntimeNetworkPolicyDisabled(t *testing.T) {
  63:func TestBuildRuntimeNetworkPolicyNoCIDRsKeepsEgressEmpty(t *testing.T) {
  76:func TestBuildRuntimeNetworkPolicyAllowsBackendAndVault(t *testing.T) {

### internal/sandbox/networkpolicy.go
  16:type NetworkPolicyMode string
  18:const (
  24:type EgressRule struct {
  31:type NetworkPolicySpec struct {
  39:func NetworkPolicySpecFromConfig(enabled bool, mode string, cidrs string) NetworkPolicySpec {
  56:func RuntimeLabels(agentID, runtime, sandboxID string) map[string]string {
  73:func BuildRuntimeNetworkPolicy(agentID, sandboxID string, spec NetworkPolicySpec) (*networkingv1.NetworkPolicy, error) {
  121:func tcpProtocol() *corev1.Protocol {
  126:func portOrNil(port int32) *intstr.IntOrString {
  131:func labelValue(value string) string {

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
  27:type SecretKeyRef struct {
  33:type AgentPodSpec struct {
  46:type ContainerSpec struct {
  63:type EnvFromSource struct {
  69:type VolumeSpec struct {
  77:type VolumeMount struct {
  83:func BuildAgentPodSpec(req AgentPodRequest) AgentPodSpec {

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
  14:type Postgres struct{ db *sql.DB }
  16:func NewPostgres(dsn string) (*Postgres, error) {
  30:func (p *Postgres) Close() error { return p.db.Close() }
  34:func (p *Postgres) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
  55:func (p *Postgres) GetUser(ctx context.Context, userID string) (domain.User, error) {
  69:func (p *Postgres) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
  83:func (p *Postgres) ListUsers(ctx context.Context) ([]domain.User, error) {
  101:func (p *Postgres) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
  136:func (p *Postgres) GetPasswordHash(ctx context.Context, username string) (string, error) {
  149:func (p *Postgres) SetPasswordHash(ctx context.Context, username, hash string) error {
  157:func (p *Postgres) CreateAgent(ctx context.Context, ownerUserID, name, runtime, model string) (domain.Agent, error) {
  176:func (p *Postgres) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
  190:func (p *Postgres) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
  208:func (p *Postgres) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
  224:func (p *Postgres) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
  242:func (p *Postgres) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
  261:func (p *Postgres) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
  275:func (p *Postgres) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
  293:func (p *Postgres) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
  333:func (p *Postgres) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
  347:func (p *Postgres) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
  365:func (p *Postgres) UpsertIntegrationConnection(ctx context.Context, connection domain.IntegrationConnection) (domain.IntegrationConnection, error) {
  385:func (p *Postgres) GetIntegrationConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  402:func (p *Postgres) DeleteIntegrationConnection(ctx context.Context, userID, providerID string) error {
  420:func (p *Postgres) UpsertAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool, revision int64, status string) (domain.AgentIntegration, error) {
  437:func (p *Postgres) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  459:func (p *Postgres) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  477:func (p *Postgres) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {

### internal/store/store_test.go
  11:func TestMemoryStore_IntegrationConnection(t *testing.T) {
  77:func TestMemoryStore_AgentIntegration(t *testing.T) {
  136:func TestMemoryStore_IntegrationConnection_UpsertRevisionAutoIncrement(t *testing.T) {

### internal/store/store.go
  14:type Store interface {
  51:var ErrNotFound = errors.New("not found")
  52:var ErrForbidden = errors.New("forbidden")
  53:var ErrConflict = errors.New("conflict")
  54:var ErrInvalidInput = errors.New("invalid input")
  56:type Memory struct {
  72:func NewMemory() *Memory {
  78:func (m *Memory) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
  102:func (m *Memory) GetUser(ctx context.Context, userID string) (domain.User, error) {
  116:func (m *Memory) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
  130:func (m *Memory) ListUsers(ctx context.Context) ([]domain.User, error) {
  141:func (m *Memory) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
  163:func (m *Memory) GetPasswordHash(_ context.Context, username string) (string, error) {
  174:func (m *Memory) SetPasswordHash(_ context.Context, username, hash string) error {
  183:func (m *Memory) CreateAgent(ctx context.Context, ownerUserID, name, runtime, model string) (domain.Agent, error) {
  208:func (m *Memory) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
  222:func (m *Memory) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
  237:func (m *Memory) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
  253:func (m *Memory) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
  271:func (m *Memory) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
  294:func (m *Memory) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
  308:func (m *Memory) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
  319:func (m *Memory) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
  345:func (m *Memory) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
  360:func (m *Memory) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
  383:func (m *Memory) UpsertIntegrationConnection(ctx context.Context, connection domain.IntegrationConnection) (domain.IntegrationConnection, error) {
  425:func (m *Memory) GetIntegrationConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
  439:func (m *Memory) DeleteIntegrationConnection(ctx context.Context, userID, providerID string) error {
  464:func (m *Memory) UpsertAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool, revision int64, status string) (domain.AgentIntegration, error) {
  502:func (m *Memory) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
  520:func (m *Memory) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
  536:func (m *Memory) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {
  574:func newID() (string, error) {

