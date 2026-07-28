package env

import (
	"context"
	"testing"
)

func TestLanguageContextOverridesProcessDefault(t *testing.T) {
	SetLanguage(LanguageChinese)
	ctx := WithLanguage(context.Background(), LanguageEnglish)

	if got := LanguageFromContext(ctx); got != LanguageEnglish {
		t.Fatalf("LanguageFromContext(ctx) = %q, want %q", got, LanguageEnglish)
	}
	if got := LanguageFromContext(context.Background()); got != LanguageChinese {
		t.Fatalf("LanguageFromContext(background) = %q, want %q", got, LanguageChinese)
	}
}
