package extension

// LanguageEnv names the environment variable holding the BCP 47 tag of the
// language lich shows its interface in when it starts the extension ("en",
// "pt-BR"). The window corrects it with a language message once it attaches,
// and sends one on every change.
const LanguageEnv = "LICH_LANGUAGE"

// LanguageData is the interface's language, sent to every running extension
// when the person changes it and when the extension starts while a window is
// attached.
type LanguageData struct {
	Tag string `json:"tag"`
}
