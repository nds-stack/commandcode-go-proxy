package proxy

import "strings"

type ModelInfo struct {
	ID      string
	Owner   string
	Aliases []string
}

var Models = []ModelInfo{
	{ID: "moonshotai/Kimi-K2.6", Owner: "moonshotai", Aliases: []string{"kimi-k2.6", "kimi2.6"}},
	{ID: "moonshotai/Kimi-K2.5", Owner: "moonshotai", Aliases: []string{"kimi-k2.5", "kimi2.5"}},
	{ID: "zai-org/GLM-5.1", Owner: "zhipuai", Aliases: []string{"glm-5.1"}},
	{ID: "zai-org/GLM-5", Owner: "zhipuai", Aliases: []string{"glm-5"}},
	{ID: "MiniMaxAI/MiniMax-M2.7", Owner: "minimaxai", Aliases: []string{"minimax-m2.7", "minimax2.7"}},
	{ID: "MiniMaxAI/MiniMax-M2.5", Owner: "minimaxai", Aliases: []string{"minimax-m2.5", "minimax2.5", "minimax"}},
	{ID: "deepseek/deepseek-v4-pro", Owner: "deepseek", Aliases: []string{"deepseek-v4-pro", "deepseek-v4", "deepseek-pro"}},
	{ID: "deepseek/deepseek-v4-flash", Owner: "deepseek", Aliases: []string{"deepseek-v4-flash", "deepseek-flash"}},
	{ID: "Qwen/Qwen3.6-Max-Preview", Owner: "qwen", Aliases: []string{"qwen-3.6-max-preview", "qwen3.6-max"}},
	{ID: "Qwen/Qwen3.6-Plus", Owner: "qwen", Aliases: []string{"qwen-3.6-plus", "qwen3.6-plus", "qwen3.6"}},
	{ID: "stepfun/Step-3.5-Flash", Owner: "stepfun", Aliases: []string{"step-3.5-flash", "step3.5"}},
	{ID: "google/gemini-3.1-flash-lite", Owner: "google", Aliases: []string{"gemini-3.1-flash-lite", "gemini-flash-lite"}},
}

// MapModel maps short alias to full model ID
func MapModel(name string) string {
	lower := strings.ToLower(name)
	for _, m := range Models {
		for _, alias := range m.Aliases {
			if alias == lower {
				return m.ID
			}
		}
	}
	return name
}
