package fonts

var (
	preferredMono  = []string{"Cascadia Mono", "Consolas"}
	preferredSans  = []string{"Segoe UI"}
	preferredEmoji = []string{"Segoe UI Emoji"}
	preferredHan   = [...][]string{
		simplifiedChinese: {"Microsoft YaHei UI", "Microsoft YaHei"},
		japanese:          {"Yu Gothic UI", "Meiryo UI", "Yu Gothic", "Meiryo"},
		korean:            {"Malgun Gothic"},
	}
)
