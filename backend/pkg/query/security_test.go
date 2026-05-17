package query

// 这些用例覆盖 docs/REVIEW-FIX-PLAN-2026-05.md 的 P1-3 / P1-4：
//   - JOIN ON 必须是结构体；旧字符串形式 → invalid_join_condition
//   - JOIN ON 的 left/right/op 全部走 ValidateIdentifier / ValidateJoinOp
//   - 普通字段名 / JSON 路径 / ORDER BY / GROUP BY / Aggregate 字段 全部过 ValidateIdentifier
//   - 即使绕过 Parser 直接调 SQLGenerator.Generate，identifier 注入仍被拒（in-depth defense）
//
// 任何这里加进来的用例都应该是"用户能从 HTTP 喂进来"的真实载荷，
// 而不是 happy path 之外的工程意外（那些走单元测试覆盖）。

import (
	"strings"
	"testing"
)

// malicious payloads —— 全部应该在 Parser/Validator/SQLGenerator 任一环被拒。
// 一旦这里有任何一条"通过"，意味着 SQL 注入面再次出现，等同 P1-3/P1-4 回归。
var maliciousIdentifierPayloads = []struct {
	name    string
	payload string
}{
	{"semicolon", "users; DROP TABLE users"},
	{"comment_dash", "id-- a"},
	{"single_quote", "users'); --"},
	{"double_quote_breakout", `id" OR "1"="1`},
	{"space", "id OR 1=1"},
	{"backtick", "`id`"},
	{"paren", "id)"},
	{"null_byte", "id\x00admin"},
	{"newline", "id\nSELECT"},
	{"star_alone", "*"},       // Select 允许 *；但在其他位置不该穿透（OrderBy / Where 等）
	{"star_qualified", "u.*"}, // 多段含 * → 段校验失败
	{"sql_keyword_punct", "1=1"},
	{"leading_digit_segment", "1abc"},
	{"dot_only", "."},
	{"trailing_dot", "users."},
	{"empty_string", ""},
}

func TestParser_Rejects_LegacyJoinOnString(t *testing.T) {
	// 防止有人把老的字符串形式悄悄从前端 / 旧 SDK 投回来。
	p := NewParser()
	_, err := p.Parse([]byte(`{
		"from":"records",
		"join":[{"type":"left","table":"users","on":"records.created_by = users.id"}]
	}`))
	if err == nil {
		t.Fatal("旧字符串形式的 join.on 必须被拒")
	}
	if !strings.Contains(err.Error(), "invalid_join_condition") {
		t.Fatalf("错误信息应包含 invalid_join_condition，实际: %v", err)
	}
}

func TestParser_RejectsMaliciousJoinOnLeftRight(t *testing.T) {
	p := NewParser()
	for _, tc := range maliciousIdentifierPayloads {
		// 跳过两个 Select 单独放行的特例（在 join.on.left/right 里它们仍然必须被拒）。
		t.Run("left/"+tc.name, func(t *testing.T) {
			body := `{
				"from":"records",
				"join":[{"type":"left","table":"users","on":{"left":` + jsonStr(tc.payload) + `,"op":"=","right":"users.id"}}]
			}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 应被拒于 join.on.left", tc.payload)
			}
		})
		t.Run("right/"+tc.name, func(t *testing.T) {
			body := `{
				"from":"records",
				"join":[{"type":"left","table":"users","on":{"left":"records.id","op":"=","right":` + jsonStr(tc.payload) + `}}]
			}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 应被拒于 join.on.right", tc.payload)
			}
		})
	}
}

func TestParser_RejectsNonWhitelistJoinOp(t *testing.T) {
	p := NewParser()
	badOps := []string{"", ";", "OR 1=1", "==", "=;DROP", "IS", "LIKE", "<", "<=", ">", ">="}
	for _, op := range badOps {
		t.Run("op="+op, func(t *testing.T) {
			body := `{
				"from":"records",
				"join":[{"type":"left","table":"users","on":{"left":"records.id","op":` + jsonStr(op) + `,"right":"users.id"}}]
			}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("op %q 不在白名单内，应被拒", op)
			}
		})
	}
}

func TestParser_RejectsMaliciousSelectFields(t *testing.T) {
	p := NewParser()
	for _, tc := range maliciousIdentifierPayloads {
		if tc.payload == "*" {
			// Select 中 "*" 是允许的（SELECT * FROM ...），不参与黑名单。
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			body := `{"from":"records","select":[` + jsonStr(tc.payload) + `]}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 select 字段校验", tc.payload)
			}
		})
	}
}

func TestParser_RejectsMaliciousOrderByFields(t *testing.T) {
	p := NewParser()
	for _, tc := range maliciousIdentifierPayloads {
		// 即便是 "*"：order by 里 * 是非法的。
		t.Run(tc.name, func(t *testing.T) {
			body := `{"from":"records","orderBy":[{"field":` + jsonStr(tc.payload) + `,"dir":"asc"}]}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 orderBy 字段校验", tc.payload)
			}
		})
	}
}

func TestParser_RejectsMaliciousGroupByFields(t *testing.T) {
	p := NewParser()
	for _, tc := range maliciousIdentifierPayloads {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"from":"records","groupBy":[` + jsonStr(tc.payload) + `]}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 groupBy 字段校验", tc.payload)
			}
		})
	}
}

func TestParser_RejectsMaliciousAggregateFields(t *testing.T) {
	p := NewParser()
	for _, tc := range maliciousIdentifierPayloads {
		// agg.Field == "*" 表示 count(*)；agg.Field == "" 也按 count(*) 处理。两者都不参与黑名单。
		if tc.payload == "*" || tc.payload == "" {
			continue
		}
		t.Run("field/"+tc.name, func(t *testing.T) {
			body := `{"from":"records","aggregate":[{"func":"sum","field":` + jsonStr(tc.payload) + `,"as":"x"}]}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 aggregate.field 校验", tc.payload)
			}
		})
		t.Run("as/"+tc.name, func(t *testing.T) {
			body := `{"from":"records","aggregate":[{"func":"sum","field":"id","as":` + jsonStr(tc.payload) + `}]}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 aggregate.as 校验", tc.payload)
			}
		})
	}
}

func TestParser_RejectsMaliciousWhereField(t *testing.T) {
	p := NewParser()
	for _, tc := range maliciousIdentifierPayloads {
		// where 用 * 没有意义；它走 validateFieldExpression，必须被拒。
		t.Run(tc.name, func(t *testing.T) {
			body := `{
				"from":"records",
				"where":{"and":[{"field":` + jsonStr(tc.payload) + `,"op":"eq","value":1}]}
			}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 where.field 校验", tc.payload)
			}
		})
	}
}

func TestParser_RejectsMaliciousJSONPath(t *testing.T) {
	// JSON path 形如 data->>name 或 data->>'name'；恶意 path 必须被段级校验拦下。
	p := NewParser()
	bad := []string{
		"data->>'name'); DROP TABLE",
		"data->>name; SELECT 1",
		"data->>$.payload",  // $ 非法
		"data->>*",          // * 非法
		"data->>a.b'.c",     // 含引号
		"data->>'a'].['b']", // 含 ]
		"data->>",           // 空 path
		"data->>1abc",       // 段首数字
		"data->>a b",        // 含空格
	}
	for _, payload := range bad {
		t.Run(payload, func(t *testing.T) {
			body := `{"from":"records","select":[` + jsonStr(payload) + `]}`
			if _, err := p.Parse([]byte(body)); err == nil {
				t.Fatalf("payload %q 不应通过 JSON path 校验", payload)
			}
		})
	}
}

func TestParser_AcceptsValidJSONPath(t *testing.T) {
	// 反例：合法的 JSON path 写法都应当通过。
	p := NewParser()
	good := []string{
		"data->>name",
		"data->>'name'",
		"data->name",
		"data.status",
		"data.payload.user.id",
	}
	for _, payload := range good {
		t.Run(payload, func(t *testing.T) {
			body := `{"from":"records","select":[` + jsonStr(payload) + `]}`
			if _, err := p.Parse([]byte(body)); err != nil {
				t.Fatalf("合法 payload %q 不应被拒: %v", payload, err)
			}
		})
	}
}

// TestSQLGenerator_InDepthDefense 即便构造方绕开 Parser 直接喂给 SQLGenerator,
// 含注入字符的标识符也必须在生成阶段被拒。这是 P1-4 的 in-depth defense 用例。
func TestSQLGenerator_InDepthDefense(t *testing.T) {
	gen := NewSQLGenerator(false)

	t.Run("malicious_select", func(t *testing.T) {
		req := &QueryRequest{
			From:   "records",
			Select: []string{"id; DROP TABLE users"},
			Page:   1,
			Size:   10,
		}
		if _, err := gen.Generate(req); err == nil {
			t.Fatal("SQLGenerator 应拒绝含分号的字段名")
		}
	})

	t.Run("malicious_join_on_left", func(t *testing.T) {
		req := &QueryRequest{
			From: "records",
			Join: []JoinClause{
				{
					Type:  "left",
					Table: "users",
					On: JoinCondition{
						Left:  "1=1) OR 1=(1",
						Op:    "=",
						Right: "users.id",
					},
				},
			},
			Page: 1,
			Size: 10,
		}
		if _, err := gen.Generate(req); err == nil {
			t.Fatal("SQLGenerator 应拒绝注入式 join.on.left")
		}
	})

	t.Run("malicious_join_op", func(t *testing.T) {
		req := &QueryRequest{
			From: "records",
			Join: []JoinClause{
				{
					Type:  "left",
					Table: "users",
					On: JoinCondition{
						Left:  "records.id",
						Op:    "OR 1=1",
						Right: "users.id",
					},
				},
			},
			Page: 1,
			Size: 10,
		}
		if _, err := gen.Generate(req); err == nil {
			t.Fatal("SQLGenerator 应拒绝白名单外的 op")
		}
	})

	t.Run("malicious_json_path", func(t *testing.T) {
		req := &QueryRequest{
			From:   "records",
			Select: []string{"data->>name'); DROP TABLE users; --"},
			Page:   1,
			Size:   10,
		}
		if _, err := gen.Generate(req); err == nil {
			t.Fatal("SQLGenerator 应拒绝注入式 JSON path")
		}
	})
}

// jsonStr 把任意字符串包成合法的 JSON 字面量，避开手工 escape 错误。
func jsonStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u00`)
				const hex = "0123456789abcdef"
				b.WriteByte(hex[r>>4])
				b.WriteByte(hex[r&0xf])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
