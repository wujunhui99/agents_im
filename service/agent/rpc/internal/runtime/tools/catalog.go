package tools

import (
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
)

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
