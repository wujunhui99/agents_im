package logic

import (
	"context"
	"reflect"
	"testing"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
)

func TestNormalizeMessageContentAtValid(t *testing.T) {
	content := `{"text":"@张三 你好","atUserList":["user_200","user_300"],"isAtSelf":false}`
	gotType, gotContent, err := normalizeMessageContent(context.Background(), nil, "usr_sender", "at", content)
	if err != nil {
		t.Fatalf("normalize at content: %v", err)
	}
	if gotType != model.ContentTypeAt {
		t.Fatalf("content type = %q, want at", gotType)
	}
	if gotContent != content {
		t.Fatalf("content = %q, want unchanged JSON", gotContent)
	}
}

func TestNormalizeMessageContentAtRejectsBad(t *testing.T) {
	cases := map[string]string{
		"invalid json":      `{"text":"hi","atUserList":[`,
		"empty text":        `{"text":"   ","atUserList":["u1"]}`,
		"missing text":      `{"atUserList":["u1"]}`,
		"empty at list":     `{"text":"hi","atUserList":[]}`,
		"at list not str":   `{"text":"hi","atUserList":[1,2]}`,
		"at list all blank": `{"text":"hi","atUserList":["","  "]}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := normalizeMessageContent(context.Background(), nil, "usr_sender", "at", content); err == nil {
				t.Fatalf("expected rejection for %s content %q", name, content)
			}
		})
	}
}

func TestExtractAtUserIDsDedupAndTrim(t *testing.T) {
	content := `{"text":"@a @b @a","atUserList":[" user_1 ","user_2","user_1",""]}`
	got := extractAtUserIDs("at", content)
	want := []string{"user_1", "user_2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extractAtUserIDs = %v, want %v", got, want)
	}
}

func TestExtractAtUserIDsNonAtIsNil(t *testing.T) {
	if got := extractAtUserIDs("text", "hello"); got != nil {
		t.Fatalf("extractAtUserIDs for text = %v, want nil", got)
	}
	if got := extractAtUserIDs("at", `{"text":"hi"}`); got != nil {
		t.Fatalf("extractAtUserIDs for invalid at (no list) = %v, want nil", got)
	}
}

func TestDecodeMessageContentPreservesAtJSON(t *testing.T) {
	atJSON := `{"text":"@张三 你好","atUserList":["user_200"]}`
	// at 消息必须整段透传给客户端（保留 atUserList 供渲染），不能被 {"text":...} 拆包。
	if got := model.DecodeMessageContent(model.ContentTypeAtValue, atJSON); got != atJSON {
		t.Fatalf("DecodeMessageContent(at) = %q, want unchanged JSON", got)
	}
	// text 消息仍拆包成纯文本。
	if got := model.DecodeMessageContent(model.ContentTypeTextValue, `{"text":"hi"}`); got != "hi" {
		t.Fatalf("DecodeMessageContent(text) = %q, want hi", got)
	}
}
