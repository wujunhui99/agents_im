package tools

import (
	"strings"

	"github.com/wujunhui99/agents_im/pkg/pythonexec"
)

func NewStaticAdapterCatalog(adapters ...ToolAdapter) AdapterCatalog {
	byToolID := make(map[string]ToolAdapter, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			continue
		}
		toolID := strings.TrimSpace(adapter.Spec().ToolID)
		if toolID == "" {
			continue
		}
		byToolID[toolID] = adapter
	}
	return AdapterCatalogFunc(func(spec ToolSpec) (ToolAdapter, bool, error) {
		adapter, ok := byToolID[strings.TrimSpace(spec.ToolID)]
		return adapter, ok, nil
	})
}

func NewDefaultLocalAdapterCatalog(pythonExecutor pythonexec.Executor, opts ...PythonExecuteAdapterOption) AdapterCatalog {
	if pythonExecutor == nil {
		pythonExecutor = pythonexec.NewDefaultExecutor()
	}
	return AdapterCatalogFunc(func(spec ToolSpec) (ToolAdapter, bool, error) {
		if !isPythonExecuteToolSpec(spec) {
			return nil, false, nil
		}
		adapter, err := NewPythonExecuteAdapter(spec, pythonExecutor, opts...)
		if err != nil {
			return nil, false, err
		}
		return adapter, true, nil
	})
}
