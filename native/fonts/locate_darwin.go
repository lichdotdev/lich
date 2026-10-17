package fonts

var (
	preferredMono  = []string{"Menlo"}
	preferredSans  = []string{"Helvetica Neue"}
	preferredEmoji = []string{"Apple Color Emoji"}
	preferredHan   = [...][]string{
		simplifiedChinese: {"PingFang SC", "Hiragino Sans GB"},
		japanese:          {"Hiragino Sans", "Hiragino Kaku Gothic ProN"},
		korean:            {"Apple SD Gothic Neo"},
	}
)
