package notify

import (
	"context"
	"errors"
	"testing"
)

func TestWebhookIsTheM2Seam(t *testing.T) {
	n := NewWebhook("feishu", "https://open.feishu.cn/open-apis/bot/v2/hook/example")
	if n.Name() != "feishu" {
		t.Fatalf("name = %q", n.Name())
	}
	err := n.Send(context.Background(), Alert{ModelID: "zai-org/GLM-4.5", Kind: "takedown"})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Send should surface the m2 seam, got %v", err)
	}
}
