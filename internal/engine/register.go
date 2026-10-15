package engine

import (
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/cleaner/ai"
	"github.com/gtkpad/devclean/internal/cleaner/bun"
	"github.com/gtkpad/devclean/internal/cleaner/cocoapods"
	"github.com/gtkpad/devclean/internal/cleaner/cpp"
	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/elixir"
	"github.com/gtkpad/devclean/internal/cleaner/flutter"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/homebrew"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/nextjs"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/pnpm"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/cleaner/terraform"
	"github.com/gtkpad/devclean/internal/cleaner/xcode"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/platform"
)

// RegisterDefaultCleaners registra todos os 18 ecossistemas suportados no motor.
func RegisterDefaultCleaners(eng *Engine, cfg *config.Config, resolver platform.Resolver) {
	RegisterDefaultCleanersWithExecutor(eng, cfg, resolver, &cleaner.DefaultExecutor{})
}

// RegisterDefaultCleanersWithExecutor permite injeção de executor customizado para testes.
func RegisterDefaultCleanersWithExecutor(eng *Engine, cfg *config.Config, resolver platform.Resolver, exec cleaner.CommandExecutor) {
	if cfg == nil {
		def := config.Defaults()
		cfg = &def
	}
	paths := resolver.Resolve()
	effectivePaths := cfg.EffectiveScanPaths()

	// Ecossistemas base de linguagens e containers
	eng.Register(docker.New())
	eng.Register(golang.New())
	eng.Register(node.New(effectivePaths))
	eng.Register(dotnet.New(paths, effectivePaths))
	eng.Register(python.New(paths.PythonUVCache, effectivePaths))
	eng.Register(rust.New(effectivePaths))
	eng.Register(java.New(effectivePaths))

	// Ecossistemas de IA, runtimes e gerenciadores de pacotes
	eng.Register(ai.New(effectivePaths))
	eng.Register(pnpm.NewWithExecutor(exec))
	eng.Register(bun.New())
	eng.Register(nextjs.New(effectivePaths))
	eng.Register(xcode.NewWithExecutor(exec))
	eng.Register(homebrew.NewWithExecutor(exec))

	// Ecossistemas de compilação, infraestrutura e mobile
	eng.Register(cpp.New(effectivePaths))
	eng.Register(flutter.New(effectivePaths))
	eng.Register(terraform.New(effectivePaths))
	eng.Register(cocoapods.New(effectivePaths))
	eng.Register(elixir.New(effectivePaths))
}

