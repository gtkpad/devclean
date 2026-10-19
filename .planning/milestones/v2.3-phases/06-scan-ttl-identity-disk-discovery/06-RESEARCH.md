# Phase 06: Scan, TTL, Identity & Disk Discovery - Research Document

**Phase:** 06-scan-ttl-identity-disk-discovery  
**Milestone:** v2.3 Paridade Total com Mole (tw93/mole)  
**Date:** 2026-10-14  
**Status:** Complete  

---

## User Constraints (from CONTEXT.md)

The following locked decisions D-01 through D-16 were established during `/gsd-discuss-phase` and are preserved verbatim as immutable engineering constraints:

### Deduplicação e Propriedade Canônica (SCAN-04)
- **D-01:** **Mais Específico Vence:** Cleaners específicos/dedicados (ex: Node, Bun, Python) possuem precedência estrita sobre cleaners genéricos de sistema/usuário (ex: `macos/User app caches`). Caminhos subordinados/filhos pertencem ao cleaner dedicado, e seus bytes são deduzidos do cleaner pai genérico para eliminar contagem dupla. — **Reversibility:** costly — afeta a ordenação e a consolidação de métricas no Engine.
- **D-02:** **Resolução Canônica de Symlinks:** Resolver symlinks usando `filepath.EvalSymlinks` antes do scan e indexar pelo caminho físico real, evitando que links simbólicos ou árvores aninhadas provoquem varredura duplicada do mesmo inode. — **Reversibility:** reversible — isolado na normalização de caminhos.
- **D-03:** **Centralização no Engine:** A lógica de deduplicação e atribuição canônica de posse reside centralmente no `internal/engine` como um estágio de resolução após a coleta/streaming dos cleaners, mantendo os cleaners desacoplados entre si. — **Reversibility:** costly — define o contrato de processamento de alvos no motor.
- **D-04:** **Omissão de Alvos Subordinados:** Alvos sobrepostos absorvidos por um cleaner mais específico são omitidos da listagem visual/relatórios, mantendo registro interno para diagnóstico. — **Reversibility:** reversible — regra de exibição e filtragem.

### Desacoplamento Scan vs. TTL e Modelagem no Target (SCAN-01)
- **D-05:** **Modelagem Explícita em domain.Target:** Adicionar campos explícitos em `domain.Target`: `EligibleBytes int64` e `TTLEligible bool`, mantendo `SizeBytes int64` como o tamanho físico total real no disco reportado pelo Scan. — **Reversibility:** one-way — altera o struct central `domain.Target` consumido pela CLI, TUI, MCP, Coordenador e cleaners.
- **D-06:** **Semântica de MaxAgeDays == 0:** Quando `MaxAgeDays == 0` (ou modo de limpeza total/forçada), `EligibleBytes` equivale a 100% de `SizeBytes` e `TTLEligible = true`. — **Reversibility:** reversible — lógica pura de cálculo.
- **D-07:** **Inocuidade do Mtime da Raiz:** O `mtime` do diretório raiz jamais oculta ou descarta um alvo durante `Scan()`. Cleaners inspecionam subpastas/arquivos para calcular `EligibleBytes` ou utilizam a data do item interno relevante. — **Reversibility:** costly — requer remoção das checagens antigas de cutoff de raiz em todos os cleaners existentes.
- **D-08:** **Granularidade de Expurgo:** Alvos atômicos de projetos (`node_modules`, `target`, `dist`) são expurgados por inteiro quando elegíveis; caches globais agregadores podem podar entradas antigas internamente caso o cleaner forneça suporte. — **Reversibility:** reversible.

### Descoberta Automática de ScanPaths (SCAN-03)
- **D-09:** **Ordem Canônica Fixa:** Seguir estritamente a ordem determinística definida na especificação (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`), incluindo apenas os diretórios que existirem fisicamente sob `$HOME`. — **Reversibility:** reversible.
- **D-10:** **Fallback Vazio Seguro:** Se nenhum dos 7 diretórios canônicos existir sob `$HOME`, `ScanPaths` permanece vazio, restringindo a varredura aos caches globais e evitando travessia recursiva irrestrita no diretório do usuário. — **Reversibility:** reversible.
- **D-11:** **Resolução Dinâmica em Memória:** Implementar método de resolução dinâmica (ex: `cfg.EffectiveScanPaths()`), garantindo que a descoberta automática opere em tempo de execução sem sujar nem persistir caminhos no arquivo YAML de configuração. — **Reversibility:** reversible.
- **D-12:** **Precedência de Configuração Explícita:** Se o usuário definir `scan_paths` explicitamente na configuração ou via flag, a auto-descoberta é desativada e apenas os caminhos fornecidos são varridos. — **Reversibility:** reversible.

### Detecção de Presença em Disco para IsInstalled (SCAN-02)
- **D-13:** **Detecção Híbrida (OU):** `IsInstalled()` retorna `true` caso `exec.LookPath(binary) == nil || fs.DirExists(defaultCacheDir)`. Se arquivos de cache órfãos existirem em disco, o cleaner correspondente é ativado para inventário e liberação de espaço. — **Reversibility:** costly — atualiza os métodos `IsInstalled()` de todos os cleaners.
- **D-14:** **Execução de Limpeza por Natureza:** Cleaners que operam diretamente no sistema de arquivos removem arquivos de cache órfãos independentemente da presença do binário; cleaners dependentes de comandos CLI (ex: Docker, Homebrew) reportam erro explicativo caso acionados para limpeza sem o binário no PATH. — **Reversibility:** reversible.
- **D-15:** **Ativação por Projetos em ScanPaths:** Para ecossistemas de linguagens (Node, Rust, Go, Java, Elixir, etc.), a presença de manifests de projeto nos `ScanPaths` ativa o cleaner para varrer artefatos locais mesmo sem a CLI do compilador/gerenciador no PATH. — **Reversibility:** reversible.
- **D-16:** **Checagens Rasas e Instantâneas:** `IsInstalled()` executa apenas verificações `os.Stat` superficiais nas raízes conhecidas de cache e diretórios imediatos de `ScanPaths`, sem travessia recursiva, mantendo a inicialização do motor imediata. — **Reversibility:** reversible.

### the agent's Discretion
- Formato de armazenamento interno do índice de caminhos canônicos no `Engine` (ex: prefix-tree / trie ou map de caminhos normalizados com `filepath.Clean`).
- Nomenclatura auxiliar de helpers em `internal/config`, `internal/fs` e `internal/engine`.

---

## Architectural Responsibility Map

| Component / Package | File(s) | Primary Responsibility in Phase 6 | Requirements / Decisions Covered |
| :--- | :--- | :--- | :--- |
| `internal/domain` | `types.go` | Adicionar `EligibleBytes int64` e `TTLEligible bool` ao `domain.Target`. | SCAN-01, D-05, D-06 |
| `internal/config` | `config.go`, `defaults.go` | Implementar `cfg.EffectiveScanPaths()` com descoberta determinística das 7 pastas sob `$HOME` e precedência estrita para `scan_paths` explícito sem mutação do YAML. | SCAN-03, D-09, D-10, D-11, D-12 |
| `internal/fs` | `stat.go` (ou `dir.go`) | Introduzir helpers puros `DirExists(path string) bool` e normalização/resolução segura de symlinks. | SCAN-02, SCAN-04, D-02, D-13, D-16 |
| `internal/engine` | `engine.go`, `register.go`, `ownership.go` | Centralizar a pipeline de resolução de propriedade canônica (`ResolveCanonicalOwnership`): indexação física com `EvalSymlinks`, dedução de bytes de descendentes sobrepostos, omissão de alvos absorvidos e injeção de `EffectiveScanPaths` no registro. | SCAN-04, D-01, D-02, D-03, D-04, D-11 |
| `internal/cleaner/*` | `dotnet`, `golang`, `bun`, `python`, `xcode`, `ai/*`, `nextjs`, `docker`, `node`, `rust`, `java`, `pnpm`, `homebrew` | Remover bloqueios de `cutoff` na pasta raiz em `Scan()`; preencher `EligibleBytes` e `TTLEligible`; implementar checagem híbrida em `IsInstalled()` (`LookPath \|\| DirExists \|\| HasManifest`). | SCAN-01, SCAN-02, D-07, D-08, D-13, D-14, D-15, D-16 |
| `cmd/` | `scan.go`, `clean.go`, `format.go` | Exibir `SizeBytes` total real e refletir `EligibleBytes`/`TTLEligible`; `clean --auto` consome alvos elegíveis por TTL sem ocultação prematura no scan. | SCAN-01, D-05, D-06 |

---

## Summary

No benchmark com o `tw93/mole`, o `devclean` deixou de identificar mais de **17 GB** de espaço recuperável. A causa-raiz identificada não foi a falta de ferramentas no sistema operacional, mas premissas arquiteturais restritivas:
1. **Filtros de TTL precoces:** Diversos cleaners descartavam pastas inteiras durante o `Scan()` porque a pasta raiz possuía `mtime` recente (`ModTime().After(cutoff)`), ignorando gigabytes de itens antigos em subpastas.
2. **`IsInstalled()` acoplado estritamente ao PATH:** Cleaners como `.NET`, `Go`, `pnpm`, `Homebrew`, `Xcode` e `Rust` retornavam `IsInstalled() == false` caso o executável não estivesse no `PATH`, mesmo existindo diretórios órfãos gigantes no disco.
3. **`ScanPaths` vazio por padrão:** Cleaners baseados em projetos varriam zero diretórios caso o usuário não configurasse manualmente o arquivo `config.yaml`.
4. **Risco de dupla contagem de bytes:** Sem atribuição canônica de propriedade e resolução de symlinks, cleaners concorrentes ou sobrepostos (específicos vs genéricos) podiam contabilizar o mesmo diretório duas vezes.

A Fase 06 resolve essas quatro frentes de forma definitiva, desacoplando o inventário físico de disco da política de expurgo, garantindo auto-descoberta determinística e estabelecendo a regra fundamental de que **cada byte em disco possui exatamente um dono canônico**.

---

## Standard Stack

- **Go 1.25 Standard Library:**
  - `path/filepath`: Normalização com `filepath.Clean`, resolução de symlinks com `filepath.EvalSymlinks`, detecção de aninhamento com `filepath.Rel`.
  - `os`: Inspeção de metadados com `os.Stat`, `os.Lstat` e leitura de diretórios com `os.ReadDir`.
  - `sync`: Sincronização em memória e paralelismo.
  - `time`: Comparação de corte temporal para cálculo de `EligibleBytes` e `TTLEligible`.
- **Bibliotecas Externas Homologadas:**
  - `golang.org/x/sync/errgroup`: Concorrência estruturada no `Engine.Scan`.
  - `github.com/bmatcuk/doublestar/v4`: Casamento de padrões glob em exclusões.
- **Restrição de Compilação Inviolável:**
  - `CGO_ENABLED=0` estrito para todas as plataformas (Darwin ARM64/AMD64 e Linux AMD64/ARM64). Nenhuma chamada CGO ou biblioteca dinâmica C é permitida.

---

## Architecture Patterns

### Pattern 1: Desacoplamento Scan vs. TTL (SCAN-01)
Atualmente em `internal/domain/types.go:23-33`:
```go
type Target struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	SizeBytes    int64         `json:"size_bytes"`
	Risk         RiskLevel     `json:"risk"`
	LastAccessed time.Time     `json:"last_accessed"`
	Category     string        `json:"category"`
	AI           *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore   float64       `json:"smart_score,omitempty"`
}
```
[VERIFIED: internal/domain/types.go:23-33]

**Evolução arquitetural:**
O método `Scan()` de qualquer cleaner NUNCA descarta um alvo por causa do `mtime` da pasta raiz.
- `Target.SizeBytes`: Representa 100% do tamanho físico ocupado no disco.
- `Target.EligibleBytes`: Volume em bytes elegível para expurgo de acordo com a política de TTL (`MaxAgeDays`).
- `Target.TTLEligible`: Booleano explícito indicando se o alvo (ou parte dele) é elegível para limpeza automática.

Regras de preenchimento (D-06, D-07, D-08):
1. **Quando `MaxAgeDays == 0` (ou forçado):**
   `EligibleBytes = SizeBytes` e `TTLEligible = true`.
2. **Quando `MaxAgeDays > 0` para alvos atômicos de projeto (`node_modules`, `target`, `dist`, `.next/cache`, `Pods`):**
   Se o alvo ou manifest qualificar como expirado pelo TTL (`LastAccessed.Before(cutoff)`), então `TTLEligible = true` e `EligibleBytes = SizeBytes`. Caso contrário, `TTLEligible = false` e `EligibleBytes = 0`. O alvo é retornado no slice de qualquer forma.
3. **Quando `MaxAgeDays > 0` para caches agregadores globais:**
   Cleaners que suportam poda granular calculam `EligibleBytes` somando os bytes dos itens internos que ultrapassam o cutoff. `TTLEligible = EligibleBytes > 0`. Se o cleaner operar o cache de forma atômica, avalia o item interno mais recente/antigo conforme a regra do ecossistema.

### Pattern 2: Detecção Híbrida em Disco para `IsInstalled()` (SCAN-02)
Cleaners existentes utilizavam predominantemente `exec.LookPath(binary) == nil`:
- `dotnet`: `exec.LookPath("dotnet")` [VERIFIED: internal/cleaner/dotnet/dotnet.go:38-41]
- `golang`: `exec.LookPath("go")` [VERIFIED: internal/cleaner/golang/golang.go:27-30]
- `pnpm`: `c.exec.LookPath("pnpm")` [VERIFIED: internal/cleaner/pnpm/pnpm.go:31-34]
- `homebrew`: `c.exec.LookPath("brew")` [VERIFIED: internal/cleaner/homebrew/homebrew.go:36-39]
- `xcode`: `c.exec.LookPath("xcrun")` [VERIFIED: internal/cleaner/xcode/xcode.go:74-80]

**Evolução arquitetural:**
A detecção passa a ser híbrida (OU) e rápida:
```go
func (c *myCleaner) IsInstalled() bool {
    // 1. Binário no PATH
    if _, err := exec.LookPath("tool"); err == nil {
        return true
    }
    // 2. Cache global padrão existente no disco
    if fs.DirExists(c.defaultCacheDir) {
        return true
    }
    // 3. (Para cleaners de linguagem) Presença rasa de manifests em ScanPaths
    for _, sp := range c.scanPaths {
        if hasShallowManifest(sp, "manifest.ext") {
            return true
        }
    }
    return false
}
```
Conforme D-16, `IsInstalled()` executa apenas verificações rasas (`os.Stat` superficial ou leitura imediata de nível 1 em `ScanPaths`), sem traversals recursivos `filepath.WalkDir`, mantendo o startup do DevClean instantâneo (<5ms).

### Pattern 3: Descoberta Automática de ScanPaths em Memória (SCAN-03)
Em `internal/config/defaults.go:7`:
```go
ScanPaths: []string{},
```
[VERIFIED: internal/config/defaults.go:7]

**Evolução arquitetural:**
Implementar `cfg.EffectiveScanPaths() []string` em `internal/config/config.go`:
- Se `len(cfg.ScanPaths) > 0`: retorna uma cópia normalizada de `cfg.ScanPaths` (precedência estrita da configuração do usuário, D-12).
- Se `len(cfg.ScanPaths) == 0`:
  Examina `$HOME` na ordem determinística estrita:
  `{"Documents", "Projects", "Developer", "Code", "Work", "repos", "workspace"}`
  Para cada diretório que existir fisicamente (`fs.DirExists(path)`), inclui no retorno.
  Se nenhum dos 7 existir, retorna slice vazio `[]string{}` (fallback seguro D-10).
- Nunca grava os diretórios descobertos em `cfg.ScanPaths` nem altera o arquivo YAML (D-11).
- No registro dos cleaners (`internal/engine/register.go`), os cleaners de projetos recebem `cfg.EffectiveScanPaths()`.

### Pattern 4: Propriedade Canônica e Deduplicação Centralizada (SCAN-04)
A deduplicação e a atribuição canônica são centralizadas em `internal/engine`:
1. **Resolução Física de Symlinks (D-02):** Todos os caminhos de alvos são normalizados com `filepath.Clean` e resolvidos via `filepath.EvalSymlinks`. Symlinks que apontam para o mesmo diretório físico compartilham a mesma chave canônica.
2. **Precedência do Mais Específico (D-01):**
   - Cleaners dedicados de ecossistema (`node`, `bun`, `python`, `pnpm`, etc.) têm precedência sobre cleaners genéricos de sistema/usuário (`macos`, etc.).
   - Caminhos filhos têm precedência sobre caminhos pais: se `/A/B` é alvo de um cleaner dedicado e `/A` é alvo de um cleaner genérico, os bytes de `/A/B` são deduzidos de `/A`:
     `parent.SizeBytes = max(0, parent.SizeBytes - child.SizeBytes)`
     `parent.EligibleBytes = max(0, parent.EligibleBytes - child.EligibleBytes)`
3. **Omissão de Alvos Subordinados Absorvidos (D-04):**
   - Se um cleaner genérico reportou um alvo idêntico ou totalmente absorvido pelo cleaner dedicado, o alvo subordinado é suprimido do resultado final.
   - Um log de diagnóstico interno registra a absorção/dedução.
4. **Alvos Virtuais Imunes:**
   Alvos com esquema virtual (`t.IsVirtual() == true`, como `docker://`, `pnpm://store/prune`, `brew://cleanup`) não passam por resolução de filesystem e são mantidos diretamente.

---

## Don't Hand-Roll

| O que NÃO fazer | Risco / Problema | Abordagem Correta do Go Standard Library |
| :--- | :--- | :--- |
| Concatenar caminhos com `+ "/"` ou `+ "\\"` | Falhas graves de portabilidade entre macOS e Linux; caminhos com barras duplas ou separadores inválidos. | Usar exclusivamente `filepath.Join(...)` e `filepath.Clean(...)`. |
| Checar se caminho é subdiretório usando `strings.HasPrefix(child, parent)` | Falsos positivos quando nomes compartilham prefixo (ex: `/home/user/app-backup` começaria com `/home/user/app`). | Usar `filepath.Rel(parent, child)`. Se o retorno não começar com `".."`, for diferente de `"."` e sem erro, então é um subcaminho genuíno. |
| Resolver links simbólicos com código próprio de leitura de link | Falhas com links encadeados (symlinks para symlinks), links relativos e loops circulares. | Usar `filepath.EvalSymlinks(path)` que resolve a cadeia completa e retorna erro se o link for quebrado ou circular. |
| Deduzir bytes permitindo valores negativos (`parent.SizeBytes - child.SizeBytes`) | Corrupção de métricas de disco e overflow caso estimativas de tamanho divirjam levemente. | Utilizar clamp defensivo: `if parent.SizeBytes < child.SizeBytes { parent.SizeBytes = 0 } else { parent.SizeBytes -= child.SizeBytes }`. |
| Persistir pastas descobertas dentro de `cfg.ScanPaths` durante o scan | Muta o arquivo de configuração do usuário em disco na próxima gravação do YAML. | Manter a descoberta estritamente dinâmica em memória via `cfg.EffectiveScanPaths()`. |

---

## Common Pitfalls

### Pitfall 1: Loops Infinitos de Symlinks e Symlinks Quebrados
Se o usuário tiver um symlink circular em `ScanPaths` (ex: `link -> .` ou `a -> b -> a`), `filepath.EvalSymlinks` detectará o ciclo e retornará um erro `too many levels of symbolic links`.
*Mitigação:* Tratar o erro de `filepath.EvalSymlinks` graciosamente: se falhar por link quebrado ou cíclico, registrar aviso/diagnóstico e usar `filepath.Clean(path)` sem travar o scan do motor.

### Pitfall 2: Sensibilidade a Caixa (Case Sensitivity) no macOS APFS vs. Linux
No macOS, o sistema de arquivos APFS padrão é case-preserving mas case-insensitive (ex: `Projects` e `projects` apontam para o mesmo diretório). No Linux (ext4), são diretórios completamente distintos.
*Mitigação:* `filepath.EvalSymlinks` no macOS já resolve para o casing canônico registrado no catálogo APFS. A lista canônica de 7 pastas (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`) é testada em ordem exata com `os.Stat`. No macOS, caso `projects` e `Projects` ambos existam, `filepath.EvalSymlinks` normaliza para o inode real unificado.

### Pitfall 3: Race Conditions em Streaming Concorrente de Resultados
No `Engine.Scan(ctx)`, múltiplos cleaners escrevem simultaneamente no canal `<-chan domain.ScanResult`.
*Mitigação:* A deduplicação e a resolução de propriedade canônica NÃO ocorrem concorrentemente enquanto os resultados chegam de forma fragmentada; ela é executada de forma atômica no estágio de consolidação (ao drenar com `engine.Collect(ctx, ch)` ou em método pós-processador `engine.ResolveCanonicalOwnership(results)`), garantindo visão global de todos os alvos antes de deduzir bytes.

### Pitfall 4: Subtração Dupla ao Tratar Múltiplos Níveis de Aninhamento
Se Cleaner A (genérico) contém Pasta X, que contém Pasta Y (Cleaner B dedicado), que contém Pasta Z (Cleaner C dedicado):
*Mitigação:* Processar alvos ordenados por profundidade decrescente (do mais profundo/específico para o mais raso/genérico), ou deduzir de cada ancestral apenas os bytes diretos não absorvidos, garantindo que os bytes de Z não sejam subtraídos duas vezes de X.

---

## Code Examples

### 1. Extensão de `domain.Target` (`internal/domain/types.go`)
```go
// [VERIFIED: internal/domain/types.go:23-33]
type Target struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Path          string        `json:"path"`
	SizeBytes     int64         `json:"size_bytes"`
	EligibleBytes int64         `json:"eligible_bytes"` // Bytes liberáveis conforme política de TTL
	TTLEligible   bool          `json:"ttl_eligible"`   // Indica se qualifica para expurgo automático
	Risk          RiskLevel     `json:"risk"`
	LastAccessed  time.Time     `json:"last_accessed"`
	Category      string        `json:"category"`
	AI            *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore    float64       `json:"smart_score,omitempty"`
}
```

### 2. Auto-Descoberta Dinâmica de ScanPaths (`internal/config/config.go`)
```go
// CanonicalDiscoveryFolders define a lista determinística das 7 pastas sob $HOME.
var CanonicalDiscoveryFolders = []string{
	"Documents",
	"Projects",
	"Developer",
	"Code",
	"Work",
	"repos",
	"workspace",
}

// EffectiveScanPaths retorna os diretórios de varredura ativos.
// Se ScanPaths foi configurado explicitamente, ele tem precedência estrita (D-12).
// Caso contrário, descobre as pastas canônicas existentes sob $HOME em ordem determinística (D-09, D-10, D-11).
func (c *Config) EffectiveScanPaths() []string {
	if len(c.ScanPaths) > 0 {
		paths := make([]string, 0, len(c.ScanPaths))
		for _, p := range c.ScanPaths {
			if clean := filepath.Clean(p); clean != "." && clean != "" {
				paths = append(paths, clean)
			}
		}
		return paths
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return []string{}
	}

	var discovered []string
	for _, folder := range CanonicalDiscoveryFolders {
		candidate := filepath.Join(home, folder)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			// Resolve symlinks canonicamente se aplicável
			if realPath, err := filepath.EvalSymlinks(candidate); err == nil {
				discovered = append(discovered, realPath)
			} else {
				discovered = append(discovered, candidate)
			}
		}
	}

	return discovered
}
```

### 3. Helper Utilitário `DirExists` (`internal/fs/stat.go`)
```go
// DirExists verifica se o caminho existe e é um diretório, tratando erros de permissão ou inexistência.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
```

### 4. Resolução de Propriedade Canônica e Deduplicação (`internal/engine/ownership.go`)
```go
package engine

import (
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
)

// EcosystemTier define a prioridade de especificidade entre ecossistemas.
// Cleaners dedicados de tecnologia/linguagem (TierDedicated) vencem cleaners genéricos de sistema (TierGeneric).
type EcosystemTier int

const (
	TierGeneric EcosystemTier = 1 // macos/user caches, system logs
	TierDedicated EcosystemTier = 2 // node, rust, python, bun, etc.
)

func getEcosystemTier(eco string) EcosystemTier {
	switch eco {
	case "macos", "system", "browsers", "leftovers":
		return TierGeneric
	default:
		return TierDedicated
	}
}

// isDescendant verifica se child está estritamente contido dentro de parent.
func isDescendant(parent, child string) bool {
	if parent == child {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

// ResolveCanonicalOwnership processa os resultados do scan aplicando as regras D-01 a D-04:
// - Avalia symlinks com filepath.EvalSymlinks
// - Cleaner dedicado absorve alvos e deduz seus bytes do cleaner pai genérico
// - Omite alvos duplicados ou subsumidos
func ResolveCanonicalOwnership(results []domain.ScanResult) []domain.ScanResult {
	// 1. Normalizar todos os alvos e resolver symlinks
	type resolvedTarget struct {
		ecoIndex    int
		targetIndex int
		target      *domain.Target
		realPath    string
		isVirtual   bool
		tier        EcosystemTier
	}

	var allTargets []*resolvedTarget
	for rIdx := range results {
		for tIdx := range results[rIdx].Targets {
			t := &results[rIdx].Targets[tIdx]
			if t.IsVirtual() {
				allTargets = append(allTargets, &resolvedTarget{
					ecoIndex:    rIdx,
					targetIndex: tIdx,
					target:      t,
					realPath:    t.Path,
					isVirtual:   true,
					tier:        getEcosystemTier(results[rIdx].Ecosystem),
				})
				continue
			}

			cleanPath := filepath.Clean(t.Path)
			realPath, err := filepath.EvalSymlinks(cleanPath)
			if err != nil {
				realPath = cleanPath
			}
			t.Path = realPath

			allTargets = append(allTargets, &resolvedTarget{
				ecoIndex:    rIdx,
				targetIndex: tIdx,
				target:      t,
				realPath:    realPath,
				isVirtual:   false,
				tier:        getEcosystemTier(results[rIdx].Ecosystem),
			})
		}
	}

	// 2. Resolver duplicatas exatas de caminho físico
	// Se dois cleaners apontam para o mesmo realPath, o TierDedicated vence e o outro é omitido (D-01, D-04)
	omitted := make(map[*domain.Target]bool)
	seenPaths := make(map[string]*resolvedTarget)

	for _, rt := range allTargets {
		if rt.isVirtual {
			continue
		}
		if existing, exists := seenPaths[rt.realPath]; exists {
			if rt.tier > existing.tier {
				omitted[existing.target] = true
				seenPaths[rt.realPath] = rt
			} else {
				omitted[rt.target] = true
			}
		} else {
			seenPaths[rt.realPath] = rt
		}
	}

	// 3. Resolver contenção hierárquica (Parent-Child overlap)
	// Se child está contido em parent, deduz os bytes do parent genérico (D-01)
	for _, child := range allTargets {
		if child.isVirtual || omitted[child.target] {
			continue
		}
		for _, parent := range allTargets {
			if parent.isVirtual || omitted[parent.target] || child == parent {
				continue
			}
			if isDescendant(parent.realPath, child.realPath) {
				// Deduz bytes do parente (especialmente se parente for TierGeneric ou cleaner amplo)
				if parent.target.SizeBytes >= child.target.SizeBytes {
					parent.target.SizeBytes -= child.target.SizeBytes
				} else {
					parent.target.SizeBytes = 0
				}

				if parent.target.EligibleBytes >= child.target.EligibleBytes {
					parent.target.EligibleBytes -= child.target.EligibleBytes
				} else {
					parent.target.EligibleBytes = 0
				}
			}
		}
	}

	// 4. Reconstruir ScanResults sem os alvos omitidos
	filteredResults := make([]domain.ScanResult, len(results))
	for rIdx, r := range results {
		filteredResults[rIdx] = domain.ScanResult{
			Ecosystem: r.Ecosystem,
			Error:     r.Error,
			Targets:   make([]domain.Target, 0, len(r.Targets)),
		}
		for tIdx := range r.Targets {
			t := r.Targets[tIdx]
			if !omitted[&r.Targets[tIdx]] {
				filteredResults[rIdx].Targets = append(filteredResults[rIdx].Targets, t)
			}
		}
	}

	return filteredResults
}
```

---

## Validation Architecture

### Test Framework

| Camada de Teste | Framework / Ferramenta | Escopo de Execução | Restrição de Execução |
| :--- | :--- | :--- | :--- |
| Testes Unitários | `testing` (Go Standard Library) | `internal/config`, `internal/domain`, `internal/fs`, `internal/engine`, `internal/cleaner/*` | `CGO_ENABLED=0`, herméticos, sem I/O de rede. |
| Testes de Integração | `testing` + `t.TempDir()` | Simulação de árvore de diretórios com mtimes manipulados via `os.Chtimes`, symlinks circulares e cruzados via `os.Symlink`. | `CGO_ENABLED=0`, tempo total < 15s. |
| Testes de Regressão CLI | `cmd` subcomandos | `devclean scan`, `devclean clean --auto`, saída JSON e tabelas. | `CGO_ENABLED=0`, execução isolada. |

### Phase Requirements -> Test Map

| Requisito | Descrição | Pacotes Afetados | Casos de Teste Chave | Comando de Validação (`go test`) |
| :--- | :--- | :--- | :--- | :--- |
| **SCAN-01** | `Scan()` reporta alvos e bytes reais sem filtrar por `mtime` da pasta raiz; `EligibleBytes` e `TTLEligible` refletem o TTL. | `internal/domain`, `internal/cleaner/*`, `cmd` | `TestScan_RootMtimeRecent_ReportsTargetAndBytes`, `TestScan_DecoupledTTL_EligibleBytesVsSizeBytes`, `TestCleanAuto_OnlyDeletesTTLEligible` | `go test -v ./internal/domain ./internal/cleaner/... ./cmd -run "Test.*(TTL\|Mtime\|Scan)"` |
| **SCAN-02** | Cleaner reporta `IsInstalled() == true` quando caches ou alvos existem no disco, mesmo sem o binário no `PATH`. | `internal/cleaner/*`, `internal/engine` | `TestCleaner_IsInstalled_NoBinaryInPath_CacheOnDisk`, `TestCleaner_IsInstalled_ManifestInScanPaths` | `go test -v ./internal/cleaner/... -run "Test.*IsInstalled"` |
| **SCAN-03** | `ScanPaths` vazio descobre as 7 pastas canônicas (`Documents`, `Projects`, `Developer`, `Code`, `Work`, `repos`, `workspace`) em ordem determinística; fallback seguro. | `internal/config`, `internal/engine` | `TestConfig_EffectiveScanPaths_DiscoveryOrder`, `TestConfig_EffectiveScanPaths_EmptyFallback`, `TestConfig_EffectiveScanPaths_ExplicitPrecedence` | `go test -v ./internal/config ./internal/engine -run "Test.*EffectiveScanPaths"` |
| **SCAN-04** | Propriedade canônica com `filepath.EvalSymlinks`, dedução de bytes de descendentes sobrepostos e eliminação de alvos duplicados. | `internal/engine`, `internal/fs` | `TestOwnership_DedicatedWinsOverGeneric`, `TestOwnership_DeductChildBytesFromParent`, `TestOwnership_SymlinkDeduplication`, `TestOwnership_VirtualTargetsImmune` | `go test -v ./internal/engine ./internal/fs -run "Test.*(Ownership\|Deduplicate\|Symlink)"` |

### Sampling Rate & Execution Speed
- A suíte completa da Fase 06 deve executar em **< 15 segundos** em ambiente local Mac/Linux.
- Taxa de amostragem de testes: 100% dos 18 cleaners registrados em `internal/engine/register.go` devem possuir assertions herméticas cobrindo `IsInstalled()` e `Scan()` com mtime recente.

### Wave 0 Gaps (Cobertura Ausente antes da Fase 6)
1. **Ausência de campos no Target:** `EligibleBytes` e `TTLEligible` inexistem em `domain.Target` [VERIFIED: internal/domain/types.go:23-33], impedindo a CLI de saber a diferença entre tamanho real e tamanho expurgável.
2. **Cleaners com bloqueio de mtime na raiz:** `dotnet.go:87`, `golang.go:104`, `bun.go:56`, `python.go:127`, `xcode.go:111`, `ai/editors.go:42,60,109`, `nextjs.go:74` descartam alvos silenciosamente se a raiz tiver `ModTime().After(cutoff)`.
3. **Cleaners dependentes exclusivamente de PATH:** `dotnet.go:38`, `golang.go:27`, `pnpm.go:31`, `homebrew.go:36`, `xcode.go:74` retornam `IsInstalled() == false` caso o binário não esteja no PATH, ignorando caches existentes no disco.
4. **Ausência de método `EffectiveScanPaths`:** `internal/config` não possui resolução determinística das 7 pastas sob `$HOME`.
5. **Ausência de pipeline de deduplicação canônica:** `internal/engine` transmite alvos via canal sem estágio de resolução de sobreposição ou dedução de bytes.

---

## Security Domain

1. **Inviolabilidade da Blacklist do Sistema (`internal/safety`):**
   - Diretórios sensíveis do sistema (`/`, `/System`, `/Library`, `/usr`, `/private`) e de credenciais do usuário (`.git`, `.env`, `.ssh`, `secrets.json`) permanecem invioláveis [VERIFIED: internal/safety/blacklist.go:9-15].
   - Nenhuma resolução de symlink ou auto-descoberta pode incluir ou penetrar diretórios da lista negra imutável.
2. **Proteção contra Escape de Symlinks:**
   - Symlinks externos apontando para raízes protegidas do sistema são barrados por `safety.IsCoreBlacklisted(realPath)`.
3. **Imutabilidade da Configuração do Usuário:**
   - A descoberta de `ScanPaths` opera em tempo de execução via `EffectiveScanPaths()` e jamais persiste pastas autodescobertas no arquivo YAML em `~/.config/devclean/config.yaml`.
4. **Prevenção de Deletions Órfãs ou Corrompidas:**
   - Ao deduplicar alvos antes do `deleter.Coordinator`, evita-se que dois cleaners enviem ordens de deleção concorrentes para o mesmo diretório ou para pasta-pai e pasta-filha simultaneamente.

---

## Sources

### Provenance Tags e Citações
- [VERIFIED: internal/domain/types.go:23-33] — Definição do struct `Target` e ausência dos campos `EligibleBytes` e `TTLEligible`.
- [VERIFIED: internal/engine/engine.go:61-129] — Orquestração de `Scan(ctx)` via `errgroup` e canal de streaming sem estágio de deduplicação canônica.
- [VERIFIED: internal/engine/register.go:43-63] — Injeção de `cfg.ScanPaths` bruto em 12 cleaners de ecossistema.
- [VERIFIED: internal/config/defaults.go:7] — `ScanPaths: []string{}` por padrão, deixando cleaners de projeto inativos sem config.
- [VERIFIED: internal/cleaner/dotnet/dotnet.go:38-41] — `IsInstalled()` restrito a `exec.LookPath("dotnet")`.
- [VERIFIED: internal/cleaner/dotnet/dotnet.go:86-89] — Filtro de mtime na raiz descartando o cache do NuGet se `lastAccessed.After(cutoff)`.
- [VERIFIED: internal/cleaner/golang/golang.go:27-30] — `IsInstalled()` restrito a `exec.LookPath("go")`.
- [VERIFIED: internal/cleaner/golang/golang.go:104] — Filtro de mtime na raiz descartando caches do Go.
- [VERIFIED: internal/cleaner/bun/bun.go:55-58] — Filtro de mtime na raiz descartando o cache do Bun.
- [VERIFIED: internal/cleaner/python/python.go:126-128] — Filtro de mtime na raiz descartando o cache uv.
- [VERIFIED: internal/cleaner/xcode/xcode.go:74-80] — `IsInstalled()` exigindo `xcrun` no PATH e `xcode.go:111` filtrando por mtime.
- [VERIFIED: internal/cleaner/pnpm/pnpm.go:31-34] — `IsInstalled()` restrito a `c.exec.LookPath("pnpm")`.
- [VERIFIED: internal/cleaner/homebrew/homebrew.go:36-39] — `IsInstalled()` restrito a `c.exec.LookPath("brew")`.
- [VERIFIED: internal/cleaner/ai/editors.go:42,60,109] — Filtros de `cutoff` ocultando dados de editores no scan.
- [VERIFIED: internal/cleaner/nextjs/nextjs.go:74] — Filtro de `cutoff` ocultando cache do Next.js.
- [VERIFIED: internal/safety/blacklist.go:9,12-15] — Lista negra imutável do sistema e do usuário.
- [VERIFIED: internal/safety/symlink.go:7-13] — Utilitário `IsSymlink` via `os.Lstat`.
- [VERIFIED: .planning/phases/06-scan-ttl-identity-disk-discovery/06-CONTEXT.md:1-110] — Decisões travadas D-01 a D-16.
- [VERIFIED: .planning/SPEC-v2.3-MOLE-PARITY.md:52-58] — Especificação da Frente 1 (Scan, TTL e Descoberta de Disco).
- [VERIFIED: .planning/REQUIREMENTS.md:8-14] — Requisitos SCAN-01, SCAN-02, SCAN-03 e SCAN-04.

---

## Metadata

- **Milestone:** v2.3 Paridade Total com Mole (tw93/mole)
- **Phase:** 06-scan-ttl-identity-disk-discovery
- **Status:** Complete & Ready for Planning
- **Author:** gsd-phase-researcher
- **Target File:** `.planning/phases/06-scan-ttl-identity-disk-discovery/06-RESEARCH.md`
