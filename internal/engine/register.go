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

	// 7 Ecossistemas originais
	eng.Register(docker.New())
	eng.Register(golang.New())
	eng.Register(node.New(cfg.ScanPaths))
	eng.Register(dotnet.New(paths, cfg.ScanPaths))
	eng.Register(python.New(paths.PythonUVCache, cfg.ScanPaths))
	eng.Register(rust.New(cfg.ScanPaths))
	eng.Register(java.New(cfg.ScanPaths))

	// 6 Ecossistemas da Fase 3
	eng.Register(ai.New(cfg.ScanPaths))
	eng.Register(pnpm.NewWithExecutor(exec))
	eng.Register(bun.New())
	eng.Register(nextjs.New(cfg.ScanPaths))
	eng.Register(xcode.NewWithExecutor(exec))
	eng.Register(homebrew.NewWithExecutor(exec))

	// 5 Novos ecossistemas da Fase 1
	eng.Register(cpp.New(cfg.ScanPaths))
	eng.Register(flutter.New(cfg.ScanPaths))
	eng.Register(terraform.New(cfg.ScanPaths))
	eng.Register(cocoapods.New(cfg.ScanPaths))
	eng.Register(elixir.New(cfg.ScanPaths))
}
