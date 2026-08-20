# Cerebrum

> OpenWolf's learning memory. Updated automatically as the AI learns from interactions.
> Do not edit manually unless correcting an error.
> Last updated: 2026-08-10

## User Preferences

<!-- How the user likes things done. Code style, tools, patterns, communication. -->

## Key Learnings

- **Project:** MCPVMWare
- **The "734/611 = 100%" coverage is 100% of the `object.*` govmomi
  wrappers, NOT 100% of the raw vim25 SOAP surface.** `gen/main.go` walked
  `object/`+`vapi/*` method sets; any managed-object method that has no
  high-level `object.*` wrapper was invisible to it. The whole iSCSI family
  fell in that gap: `object.HostStorageSystem` exposes 13 methods, none
  iSCSI, yet the raw `HostStorageSystem` SOAP MO has 16 more
  (`*InternetScsi*`, `EnableMultipathPath`/`DisableMultipathPath`/
  `SetMultipathLunPolicy`) — 72 `InternetScsi` refs in
  `referencia/govmomi/vim25/methods/methods.go`. Fixed 2026-08-19 by adding
  `generated_host_iscsi.go` (15 tools) that call
  `methods.Xxx(ctx, client.Client.Client, &types.Xxx{This: ss.Reference(), ...})`
  directly (`ss` from the existing `hostStorageSystem()` helper) — the same
  raw-`methods.*` pattern `generated_host_security.go` already used. When
  the user says "aren't there more tools for X?", check the raw vim25
  `methods.go` for the managed object, not just the `object.*` wrapper.
- **Adding a vsphere-general tool requires updating `mode_test.go`'s
  `vsphereGeneralTools` list too** — the 6 `TestMode_*` tests assert exact
  set-equality per connection mode, so a new tool that registers but isn't
  in the list (or vice-versa) fails the count/name assertions. iSCSI raised
  vsphere-general 251→266 (total 734→749); the list was updated in the same
  change.
- **`esx.ResourcePool.DisabledMethod = ["CreateVApp", "CreateChildVM_Task"]`
  — vApp creation is a genuine vCenter-only vSphere capability, not a vcsim
  gap.** Found in Fase 6: `simulator.ResourcePool.CreateVApp`/`CreateChildVM`
  fault cleanly on `simulator.ESX()`, succeed on `simulator.VPX()` — same
  "ESXi genuinely can't do this" class as the Fase 3 `HostAccountManager`
  ESX-vs-VPX asymmetry, but the opposite direction (there ESXi had MORE,
  here ESXi has LESS — don't assume a fixed direction, check each case).
- **`types.BaseComputeResourceConfigSpec`'s only interface-registered
  implementer is the generic `types.ComputeResourceConfigSpec`, but real
  callers (govc's own CLI, vcsim's `ReconfigureComputeResourceTask`) always
  pass the more specific `types.ClusterConfigSpecEx`** (which embeds the
  base and satisfies the interface via a promoted pointer method). Found in
  Fase 6 — a reminder that "the interface's registered concrete type" and
  "the type real code actually constructs" aren't always the same thing for
  `vim25/types`'s Base* polymorphic fields; check govc/vcsim usage, not just
  the interface registration in `if.go`.
- **`find.InventoryPath` is needed for almost every object NOT resolved via
  a `Finder` call — this is now the single most recurring bug class across
  every fase of this plan** (Fases 2, 3, 4, 6 all hit it independently).
  Default assumption for any new generated tool: if the returned
  `*object.Xxx`/`[]*object.Xxx` came from a property read, a `New*`
  constructor, or a create-method's Task result — NOT `client.Finder.Xxx`
  directly — assume `.InventoryPath` is `""` and verify empirically before
  trusting it in a JSON result. The one documented exception found so far:
  `Datacenter.Folders()` builds its sub-folder paths client-side already.
- **`simulator.VPX()`'s default model already creates 1 DistributedVirtualSwitch
  named "DVS0" per datacenter**, as a side effect of `Model.Portgroup: 1`
  (a DVPortgroup needs a parent DVS, so model generation creates one named
  via `m.fmtName("DVS", 0)`). Found during Fase 5 by burning real time on a
  `*types.InvalidArgument{InvalidProperty:"name"}` fault trying to create a
  *second* "DVS0" as a test fixture — the tool call itself was fine, my test
  setup was the bug. **Never create a DVS/portgroup fixture by name in a
  test against `simulator.VPX()` without first checking what the default
  model already provides** — same applies to `Model.Datastore`/`Model.Host`/
  etc., always confirm defaults before assuming a fixture needs building
  from scratch.
- **`vim25/types` has no custom `UnmarshalJSON` anywhere in the whole
  package** (confirmed by grep during Fase 4's "storage DRS" group) — the
  project's established "accept a polymorphic Base* field as generic JSON,
  decode into the common concrete struct" pattern (used throughout Fases
  2-4) only works for fields that are themselves NOT polymorphic. A truly
  polymorphic nested field (e.g. `VirtualMachineConfigSpec.DeviceChange`,
  `[]types.BaseVirtualDeviceConfigSpec`) cannot be populated through generic
  `json.Unmarshal` at all — it silently stays zero-valued/nil, no error.
  Known, accepted MVP limitation (same posture as `CustomizationSpec` in
  Fase 2) — but worth remembering as a hard architectural ceiling, not a
  per-method gap, when Fases 5-8 hit more of these.
- **`simulator.VirtualDiskManager.SetVirtualDiskUuid` is a real handler but
  a permanent stub** — its own source has `// TODO: validate uuid format
  and persist`, so it returns success without storing anything; a
  subsequent `QueryVirtualDiskUuid` never reflects the "set" value (it
  independently computes a deterministic hash-based UUID instead). Found
  during Fase 4's "virtual disk" group by testing an explicit round-trip,
  not by reading the doc comment. If any future work needs a real
  set→get UUID round trip, it cannot be verified against vcsim at all —
  would need the real host/vCenter.
- **vcsim simulator support can't be inferred from the Go receiver type name
  alone — grep by the underlying SOAP method name instead.** During Fase 3
  (Host domain codegen) I told a subagent `HostAccountManager` wasn't
  simulated based on a receiver-name-only grep; it turned out to be wrong —
  vcsim implements it under a *different* type name
  (`simulator.HostLocalAccountManager`, wired to `CreateUser`/`UpdateUser`/
  `RemoveUser`, the real SOAP method names `object.HostAccountManager.Create/
  Update/Remove` call). Confirmed by re-checking every Host receiver in this
  batch by SOAP method name instead. **Further surprise, found by the
  subagent itself while testing:** `simulator.ESX()` registers this manager
  (its static `esx/service_content.go` template sets a non-nil MOR) but
  `simulator.VPX()` does NOT (`vpx/service_content.go` leaves it nil) — the
  *opposite* of the usual "vCenter has more, ESXi has less" pattern. Also
  found (not fixed — `referencia/` is read-only vendored code): vcsim's
  `HostLocalAccountManager.UpdateUser` returns a `CreateUserBody`/
  `CreateUserResponse` instead of the matching `UpdateUser*` types, so it
  silently reports success without actually validating the update shape.
- **Any govmomi object built via a bare `NewXxx(client, ref)` constructor
  (instead of via a `Finder` method) has an empty `.InventoryPath` field —
  always resolve the real path with `find.InventoryPath(ctx,
  client.Client.Client, ref.Reference())` before returning it in a tool
  result.** This bit `vmware_vm_host_system`/`vmware_vm_resource_pool`
  (`VirtualMachine.HostSystem()`/`.ResourcePool()` both call `NewHostSystem`/
  `NewResourcePool` directly from a property value, never through a Finder) —
  see `bug-008`. **Will recur constantly in Fases 3-8**: any accessor method
  that returns `*object.HostSystem`/`*object.ResourcePool`/`*object.Datastore`/
  `*object.Folder`/etc. by constructing it from a moref (not by resolving a
  name through `client.Finder`) has this same trap. `resolveVM`/`resolveHost`
  (which DO go through the Finder) are unaffected — the bug is specific to
  objects obtained as a *return value* of another method call.
- **Never write "relies on the caller's X" as an unverified assumption in a
  tool's doc comment or design — grep for X first.** `vmware_vm_wait_for_ip`
  originally claimed "relies on the caller's MCP request timeout" for its
  unbounded `property.Wait` call; nothing in this server's stdio dispatch
  loop actually sets one (confirmed by grep, only after the assumption caused
  a real 10-minute test hang — `bug-006`). Any "wait for condition" tool in
  this project MUST take an explicit `timeout_seconds` arg with a bounded
  default (see `generated_vm_lifecycle.go`'s `defaultWaitTimeout`/
  `waitTimeoutFrom`) — there is no ambient timeout anywhere to fall back on.
- **vcsim's `SET.guest.ipAddress` ExtraConfig test fixture only populates the
  legacy `GuestNicInfo.IpAddress` field, not the newer `GuestNicInfo.IpConfig`
  that `object.VirtualMachine.WaitForNetIP` actually reads** — so
  `WaitForIP` is testable this way (it reads the legacy field) but
  `WaitForNetIP` is not (`bug-007`). No known vcsim fixture exists for
  `IpConfig` as of 2026-08-10.
- **Direct (non-Task) SOAP calls in govmomi wrap faults via `soap.WrapSoapFault`,
  NOT `soap.WrapVimFault`.** `soap.IsVimFault`/`ToVimFault` is the pair for
  `types.LocalizedMethodFault`-based **Task** errors (e.g. what
  `object.Task.Wait` surfaces); a direct method call like
  `OptionManager.Query` that faults goes through `vim25/soap/client.go`'s
  `soapRoundTrip`, which does `if f := resBody.Fault(); f != nil { return
  WrapSoapFault(f) }`. To detect a specific fault type from a direct call:
  `soap.IsSoapFault(err)` → `soap.ToSoapFault(err).VimFault()` (returns
  `types.AnyType`) → type-assert to the concrete fault **value type** (not
  pointer — confirmed by running it, `types.InvalidName` not
  `*types.InvalidName`). Found the hard way in the Fase 1 codegen pilot
  (`bug-005`) — first attempt used the Task-fault pair, compiled fine, and
  silently never matched (always returned false) until re-read
  `vim25/soap/client.go` instead of guessing. **Will matter again a lot** —
  Fases 2-8 of the codegen plan generate ~500 more direct SOAP/vapi calls,
  many of which will need this same fault-detection pattern (e.g. any
  Query/Get method that faults instead of returning empty on "no match").
- **Real vSphere behavior confirmed via vcsim source, not assumed:**
  `OptionManager.QueryOptions` faults with `InvalidName` when the `name`
  filter matches zero options — inconsistent with the `Finder` list methods
  (which return `find.NotFoundError`, already handled by
  `emptyOnNotFound`/bug-002) and with every hand-written `vmware_list_*`
  tool's "0 results = success" convention. Any future generated/hand-written
  tool wrapping a `Query*(name)`-shaped method needs the same
  fault-to-empty-result treatment — check
  `referencia/govmomi/simulator/*.go` for the real fault behavior before
  assuming empty-slice-on-no-match, it is NOT the default.
- `referencia/*` (govmomi, vsphere-automation-sdk-{go,java,python,rest},
  vmware-esxi-mcp, vmware-vcenter-mcp, vmware-vsphere-mcp-server,
  ssh-mcp-server, open-vm-tools) são **git submodules** (ver `.gitmodules` na
  raiz) — clones de terceiros só para leitura/referência, nunca importados
  diretamente; a dependência real (`govmomi`) resolve via `src/go.mod`+`go.sum`
  no módulo cache do Go. `Linux/`/`Windows/` em `referencia/` são downloads
  binários (GuestSDK tarballs/zips, VixDiskLib) sem `.gitmodules` — não são
  código-fonte a ler diretamente, são material para inspecionar via `ls`/docs.
- O código real do servidor MCP vive em `src/` (módulo
  `github.com/cslsoftwares/mcpvmware`) — `src/mcp/types.go`,
  `src/mcpvmware-mcp/main.go`, `src/tools/`, `src/vmware/`. **Não existe mais
  `cmd/`** — havia um `cmd/mcpvmware-mcp/` num scaffold anterior (visto
  09/08/2026 21:50) que foi reorganizado para `src/mcpvmware-mcp/main.go`
  algures entre 09/08 21:50 e a implementação da Fase 0 (09/08 23:08).
- [2026-08-10] **A instalação local do VMware Workstation Pro (25.0.1) traz as
  specs Swagger oficiais embutidas** em `C:\Program Files (x86)\VMware\VMware
  Workstation\swagger.zip` → `json/swagger_WS.json` (+ `swagger_Fusion.json`,
  `swagger_Player.json`), Swagger 2.0, basePath `/api`, Basic Auth, media type
  `application/vnd.vmware.vmw.rest-v1+json`. É fonte melhor que o portal
  Broadcom xAPIs ("latest" = API 1.2.1, só 23 operações; a spec do produto
  25.0.1 tem 28 — extras: restrictions, registration, params/{name},
  configparams, nicips). Atenção: os JSON do zip têm **BOM UTF-8** (strip
  `﻿` antes de `JSON.parse`). Collection Postman gerada em
  `.workspace/VMware Workstation Pro API.postman_collection.json` via conversor
  `swagger2postman.js` (scratchpad da sessão 2026-08-10).
- [2026-08-10] **vmrest local testado ao vivo (1.3.1 build 25219725)** — serviço
  configurado (`vmrest.cfg` em `%USERPROFILE%` = port 8697 + username +
  password bcrypt `$2a$14` + campo `salt` custom; formato NÃO forjável
  trivialmente) e validado com GETs reais (vms/vmnet/power/restrictions → 200;
  sem auth → 401). Flags HTTPS são minúsculas (`-c/--cert-path`,
  `-k/--key-path`); `-C` maiúsculo é `--config`. Implementação serializa
  ErrorModel com chaves capitalizadas (`{"Code":1,"Message":...}`), diferente
  da spec (minúsculas) — parse de erro deve ser case-insensitive.
- [2026-08-10] **Cobertura da API vmrest auditada por 4 vias** (spec 28/28 na
  collection, opus adversarial, portal 23⊆28, sondas ao vivo). Gotchas de
  auditoria: o mux do vmrest devolve **404 para método não registrado** (GET
  em rota PUT-only como `configparams`) — para provar existência de rota sem
  mutação, usar PUT/POST com `Content-Type: application/json` errado → 406
  "Content type not supported" responde ANTES de tocar na VM (404 = rota não
  existe de fato). Existe um namespace **interno não documentado**
  `/api/internal/...` (ex.: `GET /api/internal/vms/{id}/vmtools` → 409 com VM
  desligada) — não é API pública, não incluir em collections/tools como se
  fosse estável. Strings de rota no binário Go podem vir concatenadas/
  otimizadas — ausência de uma literal (ex.: "configparams") NÃO prova
  ausência da rota; provar sempre com sonda HTTP.
- [2026-08-12] **3 ficheiros mais errados de `vsphere-general`→`vcenter-only`
  achados na revisão prévia da Fase 7** (`extension_manager.go`,
  `custom_fields_manager.go`, `customization_spec_manager.go`) — mesmo
  padrão do bug oposto já achado na Fase 4 (`storage_resource_manager.go`/
  `namespace_manager.go`, errados na direção contrária). Confirmado por
  `ServiceContent` nil-check em `simulator/esx/service_content.go` vs
  `vpx/service_content.go`: `(*types.ManagedObjectReference)(nil)` em ESX,
  populado em VPX. **Lição reforçada:** o classificador da Fase 0 nunca
  verificou `ServiceContent` nulidade — todo domínio novo de `object/`
  precisa desta checagem explícita antes de gerar, não é opcional nem só
  para os casos "óbvios" (cluster/tenant já tinham nome sugestivo; estes 3
  não tinham nada no nome que sugerisse vCenter-only).
- [2026-08-12] **`DiagnosticManager` (Fase 7) tem ZERO simulação no vcsim**
  — confirmado por `grep -rl "DiagnosticManager\|BrowseDiagnosticLog\|
  GenerateLogBundles" referencia/govmomi/simulator/*.go` sem nenhum
  resultado (nem sequer um ficheiro `simulator/diagnostic_manager.go`
  existe). Domínio inteiro tratado como "vcsim gap, not a bug" — registado
  e testado só até `assertReachesServer`.
- [2026-08-12] **`object.Task.{Wait,WaitEx,WaitForResult,WaitForResultEx}`
  são 4 variantes da mesma operação do ponto de vista de um chamador
  MCP/JSON** — diferem só em reuso interno de `PropertyCollector`
  (thread-safety vs performance) e em `progress.Sinker` (canal Go, não
  representável em JSON). Fundidos num único tool `vmware_task_wait`
  (Fase 7) em vez de 4 tools quase-idênticas — mesmo espírito de dedup já
  aplicado a `Copy`/`Seek` de `DiagnosticLog` (fundidos num só
  `vmware_diagnostic_log_copy`, já que ambos são só loops client-side sobre
  `BrowseLog`, sem chamada SOAP própria).
- [2026-08-12] **API interna não-documentada `urn:internalvim25` existe
  também no lado SOAP** (`AuthorizationManager.DisableMethods`/
  `EnableMethods`, Fase 7) — mesmo padrão já visto no vmrest REST
  (`/api/internal/...`, ver entrada acima). No vcsim, nem sequer é
  reconhecida: falha com `ServerFaultCode: no vmomi type defined for
  'DisableMethods'` — o simulador não regista o tipo SOAP dessa API, uma
  falha mais "limpa" que um handler ausente comum. Registada mesmo assim
  (objectivo é cobertura 100%), com aviso explícito de API instável.
- [2026-08-12] **Padrão `GetXxxManager(c) (*XxxManager, error)` como guarda
  nil pronta, quando existe** — `object.GetCustomFieldsManager`/
  `object.GetExtensionManager` já devolvem `ErrNotSupported` em vez de
  fazer panic quando `ServiceContent.XxxManager` é nil; preferir esta
  função em vez do construtor `NewXxxManager` cru sempre que ela existir
  (nem todo manager tem uma — `CustomizationSpecManager`/`TenantManager`
  não têm, precisam de guarda própria manual, mesmo padrão já usado para
  `VmProvisioningChecker` na Fase 2).
- [2026-08-12] **Um subagent da Fase 7 (grupo C) achou e corrigiu um erro
  factual meu próprio no brief de delegação**: eu tinha escrito que
  `DoesCustomizationSpecExist`/`GetCustomizationSpec` "já estavam
  sem-tier" no classificador — só `GetCustomizationSpec` estava;
  `DoesCustomizationSpecExist` continuava tier2 (fail-safe, o classificador
  não reconhece prefixo "Does...Exist" como leitura). O subagent verificou
  contra o `classification.json` real em vez de confiar cegamente no meu
  brief, e corrigiu sozinho — reforça a lição já registada de nunca aceitar
  uma alegação (minha ou de um agent) sem verificação própria contra a
  fonte.
- [2026-08-12] **Achado real fora do escopo da Fase 7, sinalizado para
  follow-up:** `generated_vm_provisioning.go` (Fase 2)'s `handleVMCustomize`
  alega no seu próprio comentário que o decode genérico funciona para
  `types.CustomizationSpec.Identity` — o subagent do grupo C da Fase 7
  provou empiricamente que isso é FALSO (`json: cannot unmarshal object
  into Go struct field CustomizationSpec.identity of type
  types.BaseCustomizationIdentitySettings` — é um campo polimórfico
  aninhado, mesma limitação arquitectural já documentada desde a Fase 4).
  `vmware_vm_customize` não consegue hoje transportar uma customização
  real. Não corrigido nesta fase (fora do ficheiro do grupo C) — pendência
  aberta, ver plano/report da Fase 7.

- [2026-08-12] **`vcsim` tem um simulador REST/VAPI SEPARADO** — `github.com/vmware/govmomi/vapi/simulator`
  (não confundir com o pacote `simulator` core usado desde a Fase 0), nunca
  importado neste projecto antes da Fase 8a. Cobre `vapi/tags`,
  `vapi/library` (Content Library) e `vapi/vcenter` (templates/OVF) — mas
  só arranca se: (1) blank-import `_ "github.com/vmware/govmomi/vapi/
  simulator"` (regista-se via `init()`+`simulator.RegisterEndpoint`); E
  (2) `model.Service.RegisterEndpoints = true` **antes** de
  `model.Service.NewServer()` (sem isto, toda rota REST devolve 404 limpo,
  não um erro óbvio de "simulador não suporta"). `simulator.Model.Run()` já
  faz isto internamente — o harness próprio deste projecto (`newSimClient`
  em `testhelpers_test.go`) nunca passou por `Model.Run`, por isso nunca
  tinha sido descoberto. Confirmado com spike próprio (não assumido) antes
  de delegar aos subagents da Fase 8a — aplicado uma única vez em
  `testhelpers_test.go`, todos os grupos delegados beneficiam sem repetir a
  configuração. **Domínios vapi/* SEM cobertura neste simulador** (namespace,
  cluster, esx/settings, crypto, cis/tasks, authentication, appliance/*
  pequenos, vm/dataset) continuam "vcsim gap, not a bug" — confirmado por
  ausência do import correspondente em `vapi/simulator/simulator.go`.
- [2026-08-12] **`vapi/*` (REST) tem `json` tags nativas — ao contrário de
  `vim25/types` (SOAP/XML) usado por `object/`.** Isto elimina, para todo o
  domínio REST, a necessidade do padrão `decodeJSONArg`/decode genérico
  criado desde a Fase 1 para specs polimórficos SOAP — `json.Unmarshal`
  directo para o struct concreto (`library.Library`, `tags.Tag`, etc.)
  funciona sem workaround. Diferença arquitectural relevante pra qualquer
  fase futura que toque `vapi/*`.
- [2026-08-12] **`vapi/rest.Client` (11 métodos: Login/Logout/Session/Do/
  Download*/Upload/WithHeader/WithSigner) excluído inteiro da Fase 8a** —
  é a camada de transporte/sessão interna, já gerida por
  `vmware.Client.REST(ctx)` desde a Fase 4; expor como tools deixaria um
  chamador MCP fazer logout da sessão REST partilhada por todas as outras
  tools `vapi/*`, ou disparar pedidos HTTP arbitrários. Mesma classe de
  exclusão já aplicada a `HostConfigManager` (Fase 3) e `DiagnosticManager.
  Log` (Fase 7) — nunca expor plumbing interno como tool standalone.

- [2026-08-12] **`referencia/govmomi` (checkout usado pra LER código desde a Fase 0)
  pode ter drift real contra a versão pinada em `go.mod`** (`v0.55.1`, sem
  `replace`) — achado na Fase 8a (`bug-012`): `vapi/esx/settings/clusters/
  vms/transition.go` tem um método (`DeleteSolutionOnly`) em `referencia/`
  que não existe na versão pinada de facto compilada. **O gate obrigatório
  de `go build` limpo por fase já apanha isto automaticamente** — nenhuma
  fase anterior pode ter "escorregado" um método inexistente sem o build
  falhar da mesma forma. Não precisa de auditoria retroactiva; é só mais um
  motivo pra nunca pular o gate de build antes de reportar uma fase pronta.
- [2026-08-12] **Erro de handler não aparece no campo `error` do JSON-RPC
  top-level** — o `mcp` deste projecto embrulha erros de tool dentro de
  `result.content[].text` + `result.isError:true` (convenção MCP), não no
  `error` do JSON-RPC. Um script de smoke/verificação que só cheque
  `resp["error"]` ou a ausência de `resp["result"]` para decidir
  sucesso/falha está a verificar a coisa errada — sempre inspeccionar
  `result.isError` quando quiser distinguir "tool chegou ao servidor e
  devolveu um erro real" de "tool teve sucesso".
- [2026-08-12] **1 caso de sub-classificação (não sobre-classificação) achado
  na Fase 8a** — `vapi/authentication.Manager.Issue` (emite um token de
  autenticação via POST real) tinha ficado **sem-tier** no classificador da
  Fase 0 (o regex de leitura bateu por engano, "Issue" não tem verbo
  mutante reconhecido no prefixo). Corrigido pra tier2. Até aqui todas as
  correcções de tier desta fase (Fases 0-8a) tinham sido na direcção
  oposta (tier2→sem-tier); primeiro caso confirmado do erro inverso —
  reforça que o fail-safe do gerador não é infalível em NENHUMA direcção,
  vale sempre ler o source antes de aceitar a classificação automática.

- [2026-08-12] **`/api/...` vs `/rest/...` são 2 gerações de API REST/VAPI
  distintas para a mesma capacidade lógica** — `vapi/rest.Client.Resource()`
  (`isAPI(path)`) usa o path como está se começar por `/api` (SDK Go
  moderno, ex. `vapi/appliance/access/consolecli.Path = "/api/appliance/
  access/consolecli"`), mas prefixa com `Path` (`/rest`) tudo o resto
  (collection Postman legacy, ex. `/appliance/access/consolecli` →
  `/rest/appliance/access/consolecli`). São endpoints DIFERENTES, não
  duplicados — descoberto na Fase 8b ao tentar deduplicar contra a Fase 8a,
  a minha primeira suposição (mesma capacidade = mesmo endpoint) estava
  errada. Antes de excluir algo como "já coberto", confirmar o path literal,
  não só a capacidade lógica.
- [2026-08-12] **Técnica de geração sem fonte Go (Fase 8b) é viável e já
  tem precedente** — quando não há wrapper Go nenhum (ex. VAMI legacy
  `/rest/appliance/*`), parse estruturado (Python) de uma collection
  Postman vendorizada + classificação de tier manual (verbo HTTP+semântica
  do nome da rota, sem fonte pra confirmar comportamento real) + decode
  genérico `interface{}` (mesmo padrão já usado à mão desde a Fase 4 em
  `tools/appliance.go`) é suficiente pra gerar tools reais e testáveis
  (via fixture httptest, nunca vcsim — não simula nada disto). Aplicável a
  qualquer domínio futuro sem SDK Go tipado.

- [2026-08-12] **Padrão pra adicionar um 2º tipo de cliente ao `Registry`
  sem tocar tools existentes** (Fase 9, `workstation.Client`): `Tool` ganha
  um campo novo (`WSHandler`) em vez de alargar o tipo de `client` pra
  `interface{}` — `register()` original fica intocado, um `registerWorkstation()`
  irmão constrói `Tool{WSHandler: fn}`, `CallTool` despacha por qual campo
  está preenchido. `RegistryOptions` ganha um campo NOVO opcional
  (`WorkstationClient`), nunca um parâmetro posicional novo em
  `NewRegistry` — assim `RegistryOptions{}` continua a significar
  exactamente o que significava antes, e nenhum dos ~30 ficheiros de
  teste que já chamavam `NewRegistry(ctx, client, RegistryOptions{...})`
  precisou de mudar. Reutilizável pra Fase 10 (VMC on AWS, outro cliente
  novo).
- [2026-08-12] **`vmrest` (Workstation Pro) tem 1 rota com corpo cru, não
  JSON**: `PUT /vms/{id}/power` — o corpo é literalmente a string `on`/
  `off`/`shutdown`/`suspend`/`pause`/`unpause` (sem aspas), apesar do
  `Content-Type` continuar a ser o media type vmrest normal
  (`application/vnd.vmware.vmw.rest-v1+json`). Confirmado lendo a
  definição real do request na collection Postman, não assumido — exigiu
  um método `DoRawBody` separado de `Do` no cliente novo.
- [2026-08-12] **Um campo de `connectionModeAllows` escrito
  especulativamente antes de qualquer tool existir pode ficar
  inconsistente quando a fase chega** — `ConnectionModeAll` já incluía
  `modeWorkstation` desde 10/08/2026 (antecipando a Fase 9), mas quando a
  Fase 9 realmente chegou, `--vmware-all-url` só constrói 1 cliente
  (`*vmware.Client`), então deixar isso passar anunciaria 28 tools que
  falhariam sempre. Corrigido pra vSphere-only, com nota explícita de que
  é uma decisão em aberto (2 clientes vivos simultâneos), não resolvida
  silenciosamente. Reconferir qualquer decisão especulativa antiga contra
  a realidade assim que a fase que a motivou chega, não confiar cegamente
  no código antigo.

- [2026-08-12] **Padrão de N-ésimo cliente no `Registry` consolidado**
  (Fase 10, 3º cliente `cloudaws.Client`, mesma técnica já usada pra
  `workstation.Client` na Fase 9) — `Tool` ganha mais um campo
  (`CloudHandler`), `RegistryOptions` ganha mais um campo opcional
  (`CloudAWSClient`), `registerCloudAWS`/`registerDestructiveCloudAWS`
  espelham os pares já existentes. Zero ficheiros de teste mudaram outra
  vez. Este padrão está agora provado 2x — é o caminho certo pra qualquer
  produto futuro que precise de um cliente/protocolo de auth diferente.
- [2026-08-12] **Domínio com risco financeiro real exige tier mais
  cauteloso que a convenção padrão do projecto** (Fase 10, VMC on AWS) —
  a convenção normal (GET=sem-tier, DELETE=tier1, resto=tier2) assume que
  "tier1=irreversível" é sobre PERDA DE DADOS/CONFIG. Quando a operação
  custa dinheiro real por hora enquanto existir (SDDC na AWS), até um
  `POST .../sddcs` (create) ou `PATCH` (update/resize) precisa de tier1,
  não só `DELETE` — decisão tomada explicitamente pelo orquestrador antes
  de delegar, não deixada pros subagents inferirem sozinhos. Aplicável a
  qualquer domínio futuro com custo de infra real (ex.: se este projecto
  algum dia cobrir outro cloud provider).
- [2026-08-12] **Zero colisões de build entre 4 grupos paralelos** (Fase
  10) — melhor resultado da sessão inteira (todas as fases anteriores com
  ≥2 grupos tiveram pelo menos 1 colisão transiente de símbolo/nome,
  sempre auto-resolvida). Cada grupo prefixou os seus helpers internos
  com um nome único (`cloudAWSSDDC*`, `cloudAWSNetCore*`, etc.) por
  precaução própria, sem instrução explícita minha pra isso — reforça que
  vale a pena incluir essa recomendação explicitamente em briefings
  futuros com ≥3 grupos paralelos no mesmo pacote Go.
- [2026-08-12] **Nunca deixar um smoke test automatizado chamar uma tool
  sem gate contra um serviço de produção real de terceiros** — o binário
  `mcpvmware-mcp.exe` não tem flag pra redireccionar os hosts CSP/VMC
  (`console.cloud.vmware.com`/`vmc.vmware.com`) pra um fixture local; o
  smoke da Fase 10 chamou só uma tool tier1 SEM `--allow-destructive`
  (gate fecha antes de qualquer rede) para provar o wiring, e
  deliberadamente NÃO chamou nenhuma tool sem-tier (que teria ido direto
  à rede com um token falso). Aplicável a qualquer domínio futuro cujo
  cliente aponte pra um host de produção fixo (não localhost/vcsim).

## Do-Not-Repeat

- [2026-08-19] **Guest-op test travava (~9 min) — causa REAL: bug de deadlock
  no vcsim, NÃO socket exhaustion (minha 1ª hipótese, REFUTADA por evidência ao
  ler a fonte do simulador — registrado como lição: não fixar a hipótese fácil).**
  `simulator/guest_operations_manager.go` `StartProgramInGuest` contra VM sem
  container backing (`vm.svm==nil`) desreferencia `vm.svm.c.id` sem `return` →
  panic nil server-side; e `simulator/registry.go` `WithLock` faz `f(); unlock()`
  SEM `defer`, então o panic (recuperado inofensivamente pelo net/http) deixa o
  `ObjectLock` da VM travado PARA SEMPRE → qualquer guest-op seguinte na MESMA VM
  deadlocka. Flaky por ordem de iteração de map (passa isolado). **Mitigação no
  teste (o código sob teste está correto — é bug do vcsim vendorizado, read-only):**
  (1) 1 vcsim compartilhado por arquivo de teste com subtests `t.Run` — boa
  prática geral de qualquer forma; (2) drivar `start_program` POR ÚLTIMO numa VM
  descartável dedicada (`simulator.ESX()` tem `Machine:2`), isolando o lock
  envenenado. Lição dupla: (a) NÃO confiar em relatório vago de subagente
  (correr+ler eu mesmo pegou o travamento que o "vou esperar" escondia);
  (b) o subagente refutou minha teoria lendo a fonte — provar, não assumir.
- [2026-08-19] **Gate `go test ./...` no Windows FALHA por flaky de contenção
  quando roda os pacotes EM PARALELO (default): `tools` (muitos vcsim) +
  `vmware` (mais vcsim) concorrem por portas/sockets e o `vmware` dá FAIL/timeout
  (~22s) — mas cada pacote passa ISOLADO (`tools` ~89s, `vmware` ~4s). Não é
  regressão.** SEMPRE rodar o gate da suíte completa com `-p 1` (serial, um
  pacote por vez): `go test ./... -p 1 -count=1`. Aplicar em toda onda antes de
  commitar; nunca concluir "FAIL" sem antes tentar isolado/serial.
- [2026-08-19] **`-p 1` REDUZ mas NÃO ELIMINA o flaky de contenção no Windows —
  o `workstation` (que eu nem toquei) deu FAIL de conexão mesmo com `-p 1`, e
  passou ISOLADO (1.3s).** Gate REALMENTE confiável = rodar CADA pacote em
  PROCESSO SEPARADO: `for p in tools cloudaws vmware workstation; do go test
  ./$p/ -count=1 -timeout 200s; echo "$p exit $?"; done`. **Erro de processo que
  cometi (não repetir):** juntei `go test ./... | tail; git add; git commit` no
  MESMO comando bash — o "exit 0" que li era do `echo` final, não do `go test`
  (que deu 1), e commitei com um FAIL na tela sem perceber. SEMPRE rodar o gate
  num comando, LER o resultado, e só então commitar num comando SEPARADO.
- [2026-08-19] **Mesmo o gate por-pacote flaka DENTRO do `tools`: o pacote tem
  ~400 testes que cada um sobe um vcsim (httptest TLS) — num único processo eles
  competem por portas no Windows e ~1 a cada 2-3 runs um punhado FALHA com "dial
  tcp / TLS handshake / connection failed". Os mais comuns: `TestVMLifecycleTools_UpgradeVM`,
  `_Unregister`, `_UnsimulatedMethods` (todos passam ISOLADOS em ~3s).** Piora a
  cada onda (mais tools = mais testes = mais vcsim). Gate CONFIÁVEL do `tools` =
  RETRY: `for i in 1 2 3; do go test ./tools/ -count=1 -timeout 180s && break; done`.
  Se um FAIL for só nesses testes-vcsim não-tocados com erro de conexão, é
  ambiente (provar rodando o teste isolado), NÃO regressão — mas obter o verde
  limpo por retry antes de commitar em vez de commitar com FAIL na tela.
- [2026-08-11] **A regra global "atualizar o plano por fase/onda" (pendências
  NUMERADAS, estado marcado concluído/pendente/bloqueado, comandos+resultado
  literais, nota de commit mesmo quando não há) precisa de ser aplicada como
  checklist literal, não só "no espírito".** A minha 1ª versão da secção
  "Fase 2 executada" tinha todo o CONTEÚDO certo (o quê deu certo/errado,
  soluções, achados) mas falhava 4 itens formais da regra: pendências em
  parágrafo corrido em vez de lista numerada; sem tag de estado explícito;
  sem nota "sem commits"; verificação em prosa/negrito em vez de bloco de
  comando literal. O usuário perguntou directamente "fez do jeito que
  gosto?" e eu tive de reler a própria regra pra achar os gaps — não bastou
  "lembrar" que a regra existe, tive de checar item a item. **Pra próxima
  actualização de plano: passar pelos 5 itens da regra como checklist antes
  de considerar a secção pronta**, não confiar em "escrevi bastante detalhe,
  deve estar bom".
- [2026-08-11] **"Reutilizar o que já existe" aplicado de forma rasa —
  copiar o padrão de um ficheiro vizinho em vez de ler a regra-fonte que o
  define — deixa passar metade da convenção.** Segui correctamente a
  nomenclatura de planos (`.workspace/plans/<NomeProjeto>YYYY-MM-DD-HHMMSS-
  <slug>.plan.md`) porque **imitei** um `.plan.md` já existente no repo,
  mas nunca abri `.cursor/rules/workspace-plans-persist_V1.4.0.mdc` — a
  mesma regra que também define `.workspace/reports/.../<slug>.report.md`
  como par irmão. Como nenhum `.report.md` existia ainda pra eu copiar, e
  como o `CLAUDE.md` da raiz manda seguir o protocolo OpenWolf (`.wolf/*`)
  "toda sessão" (que nunca menciona `.workspace/reports/`), os resultados
  da Fase 2 (VM codegen) acabaram espalhados só por `.wolf/STATUS.md`/
  `cerebrum.md`/`buglog.json` + o plano — nunca um report formal. Achado
  porque o usuário perguntou "onde colocou o report" e eu tive de investigar
  a regra a sério (delegado a um Explore agent, depois confirmado por mim
  lendo o `.mdc` completo). **Regra pra próxima vez:** quando um padrão de
  nomenclatura/local for inferido por imitação de um ficheiro existente,
  ainda vale a pena localizar e ler a regra-fonte — ela pode definir mais
  do que o exemplo copiado mostra.
- [2026-08-10] **Um background Agent que devolve um relatório final vago
  ("I'll pause here and wait for the background test run to finish")
  precisa de verificação independente antes de aceitar como concluído** — o
  agent do grupo "lifecycle" da Fase 2 (VM codegen) terminou assim porque o
  seu próprio `go test` interno tinha ficado preso 10 minutos num teste real
  (`bug-006`, `wait_for_net_ip`) e o turn dele acabou antes de reportar. Os
  ficheiros que ele produziu estavam reais e substanciais (37KB+30KB), mas
  eu só descobri isso a mais (e os 4 bugs reais escondidos atrás do hang —
  `bug-006`, `bug-007`, `bug-008`) correndo eu mesmo `go test ./tools/...
  -timeout 120s` de novo, não confiando no relatório do agent. Os outros 2
  agents da mesma leva (device, provisioning) devolveram relatórios
  completos e detalhados — confirma que relatório incompleto É sinal de
  problema, não estilo/variação normal.
- [2026-08-10] **Uma nota no plano dizendo "o gerador pula nomes já
  registados" (`gen/main.go`, Fase 0 do plano de codegen) estava errada** —
  nunca foi implementado, era uma suposição minha escrita como se fosse
  facto observado. Só descoberta na revisão da Fase 0 (19:48) porque
  verifiquei com `grep -i "existing\|dedup\|skip.*regist" gen/main.go` e deu
  zero resultados — achei 8 colisões reais de nome entre o relatório gerado
  e as 29 tools hand-written (`vmware_vm_destroy`, `vmware_vm_power_on`,
  etc.). **Nunca escrever no plano/STATUS que um mecanismo "já existe" ou
  "já trata X" sem grep/leitura confirmando — mesmo dentro da mesma sessão,
  uma frase escrita de cabeça vira uma mentira que se propaga pros relatórios
  seguintes se não for verificada.**
- [2026-08-10] O plano de implementação
  (`.workspace/plans/MCPVMWare2026-08-09-224451-plano-implementacao-tools-mcp.plan.md`)
  tinha várias referências a caminhos internos do govmomi escritas como
  `src/session/keepalive/handler.go`, `src/object/task.go`,
  `src/vapi/rest/client.go`, `src/simulator/model.go`, `src/vcsim`,
  `src/vapi/appliance/*` — herdadas de antes da reorg de pastas, quando "src"
  ainda não era o nome do diretório do próprio projeto. Ficaram ambíguas/erradas
  depois que `referencia/govmomi` e `src/` (projeto) passaram a coexistir.
  Corrigido nesta revisão para `referencia/govmomi/...`. Ao ler/escrever
  planos ou docs deste projeto, sempre distinguir explicitamente as duas
  árvores — nunca escrever "src/" a secas quando o alvo é código de terceiros
  em `referencia/`.
- [2026-08-10] **`pdftoppm` (poppler) NÃO está instalado nesta máquina** — a
  ferramenta Read falha em qualquer PDF com `"pdftoppm is not installed..."`
  (confirmado tentando ler `referencia/vmware-vsphere-7-0.pdf` directamente).
  **2 de 2 subagentes Haiku instruídos a "ler o PDF via Read" bateram no mesmo
  erro e, em vez de reportar a falha, inventaram sumários plausíveis mas
  falsos** (contagem de páginas, estrutura de capítulos) para
  `vmware-vsphere-sdks-and-tools-{6-5,6-7,7-0,9-1}.pdf` — só um 3º agente
  (para `vmware-vsphere-7-0.pdf`) reportou a falha honestamente. Alternativa
  que funciona: `pdftotext` **está** instalado (`/mingw64/bin/pdftotext`,
  parte do mesmo poppler mas só o binário de extração de texto) — extrair via
  Bash (`pdftotext -layout arquivo.pdf saida.txt`) para um `.txt` no
  scratchpad e ler/grep esse `.txt` (Read/Grep funcionam normalmente em texto
  simples). **Nunca pedir a um subagent para "ler um PDF" nesta máquina sem
  pré-extrair com `pdftotext` primeiro** — o risco de alucinação silenciosa é
  real e já se confirmou 2x na mesma leva de agentes.
- [2026-08-10] **`tools.newSimClient` (harness vcsim, Fase 0) tinha um bug de
  duplo-login nunca detectado** — só foi exercitado pela primeira vez ao
  escrever `inventory_test.go` (Fase 2), `client_keepalive_test.go` usa um
  caminho diferente (`simulator.Test`). Causa: `s.URL` (devolvido por
  `model.Service.NewServer()`) já vem com `user:pass@` embutido (as
  credenciais default do vcsim); passar isso direto pra `vmware.Config.URL`
  faz o `govmomi.NewClient` fazer auto-login sozinho (por a URL ter
  userinfo — ver doc do próprio `NewClient`), e o `Login` explícito que vem a
  seguir falha ("Login failure") porque já existe sessão. Fix: `u := *s.URL;
  u.User = nil` antes de montar o Config. Ver `.wolf/buglog.json` bug-001.
  **Sempre que criar um novo `*_test.go` que usa `newSimClient` pela
  primeira vez, correr `go test` cedo** — helpers de harness só ficam
  provados quando um teste real os exercita, não quando compilam.
- [2026-08-10] **`client.Finder` sem datacenter default (vCenter com mais de
  1 datacenter) quebra qualquer list method com um path relativo tipo "*"**
  — `vmware.NewClient` só chama `SetDatacenter` quando `DefaultDatacenter`
  resolve sem ambiguidade (ESXi standalone, ou vCenter com exactamente 1
  datacenter); com 2+ datacenters fica `nil`, e `HostSystemList`/
  `DatastoreList`/`NetworkList`/`ResourcePoolList`/
  `ClusterComputeResourceList`/`VirtualMachineList` erroram "please specify a
  datacenter" nesse caso (confirmado por spike + `inventory_test.go` contra
  `simulator.VPX()` com `Model.Datacenter=2`, não assumido). Fix: usar um
  path absoluto com o datacenter em wildcard (`/*/host/*`, `/*/vm/*`, etc. —
  ver `tools/finder.go:dcScopedPath`) em vez de um path relativo tipo "*".
  Combinar sempre com `emptyOnNotFound` — os mesmos list methods devolvem
  `find.NotFoundError` (não uma lista vazia) quando 0 resultados batem (ex.:
  clusters num ESXi standalone). Ver `.wolf/buglog.json` bug-002. Isto era um
  bug latente já no `vmware_list_vms` (Fase 0, existia antes desta sessão) —
  nunca detectado porque o único alvo real de teste (`10.100.2.54`) é ESXi
  standalone, que sempre resolve `DefaultDatacenter` sem ambiguidade.

- [2026-08-10] **vSphere `Destroy_Task` rejects a powered-on VM** (`InvalidPowerState`
  fault) — confirmed against vcsim while writing `TestVMTools_Destroy`
  (`tools/vm_test.go`). `vmware_vm_destroy` deliberately does **not**
  auto-power-off as a side effect (a Tier 1 tool silently doing extra
  destructive work on your behalf is worse, not safer) — the caller must
  `vmware_vm_power_off` first; the tool's description says so explicitly.
- [2026-08-10] `VirtualMachineConfigSpec.NumCPUs`/`MemoryMB` are plain
  `int32`/`int64` (not pointers) but tagged `omitempty` in
  `referencia/govmomi/vim25/types/types.go` — leaving one at its Go zero
  value and only setting the other in `Reconfigure` is safe (the zero field
  is omitted from the XML payload, vSphere treats it as "no change"), not a
  footgun that resets it to 0. Verified with a test
  (`TestVMTools_Reconfigure`), not assumed from reading the tag alone.
- [2026-08-10] **`vmware.Client.REST(ctx)` (VAMI accessor) só depende do
  `*vim25.Client` SOAP embutido — nunca toca `ServiceContent` nem faz
  qualquer round-trip SOAP.** Isso permite testar tools de VAMI (Fase 4)
  contra um fixture `httptest.Server` puro, **sem vcsim/simulador SOAP
  nenhum**: `&vmware.Client{Client: &govmomi.Client{Client: &vim25.Client{
  Client: soap.NewClient(fixtureURL, true)}}}` — os campos privados `cfg`
  ficam zero-value (username/password vazios), inofensivo porque o fixture
  controla a resposta do login. Padrão em `tools/appliance_test.go` — útil
  para qualquer tool futuro que só use `client.REST(ctx)`, não
  `client.Finder`/SOAP.
- [2026-08-10] Endpoints VAMI reais confirmados por **parse estruturado
  (Python `json.load`) da própria collection Postman já vendorizada**
  (`.workspace/vSphere Automation REST Resources for
  appliance.postman_collection.json`) em vez de adivinhados: `GET
  rest/appliance/system/version`, `rest/appliance/system/uptime`,
  `rest/appliance/health/{system,applmgmt,database-storage,load,mem,
  software-packages,storage,swap}` — 8 subsistemas de health, não um número
  arbitrário. Útil se mais tools de VAMI entrarem em escopo — a collection já
  tem o resto das ~130 rotas catalogadas.

## Decision Log

<!-- Significant technical decisions with rationale. Why X was chosen over Y. -->
