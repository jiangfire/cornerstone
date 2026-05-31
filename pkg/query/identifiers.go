package query

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxIdentifierLength 单段标识符的最大字符数。
const MaxIdentifierLength = 128

// allowedJoinOps Join.On 仅允许这两个比较符；
// 其他运算符无明确等价语义并显著放大注入面，一律拒绝。
var allowedJoinOps = map[string]struct{}{
	"=":  {},
	"<>": {},
}

// identifierSegmentPattern 单段标识符：必须以字母/下划线起头，仅含字母数字下划线。
var identifierSegmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsonPathSegmentPattern 与标识符同规则；JSON path 不允许带引号、通配符、空格等。
var jsonPathSegmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateIdentifier 校验"表名 / 字段名 / 限定字段"。
// 允许形如 `table.field`、`alias.field.sub` 的多段引用；每段独立校验。
//
// 拒绝：含单/双引号、分号、空格、注释符、SQL 关键字标点等。
// 设计意图：所有进入 SQL 拼接（无论是否经过 quoteIdentifier）的标识符都先过此校验，
// 从而即便后续 quoteIdentifier 出现遗漏，也无法构造出能破出引号的字面量。
func ValidateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("标识符不能为空")
	}
	if len(name) > MaxIdentifierLength*8 {
		return fmt.Errorf("标识符过长（含分隔符上限 %d 段）", MaxIdentifierLength)
	}
	for segment := range strings.SplitSeq(name, ".") {
		if err := validateIdentifierSegment(segment); err != nil {
			return fmt.Errorf("非法标识符 %q：%w", name, err)
		}
	}
	return nil
}

func validateIdentifierSegment(seg string) error {
	if seg == "" {
		return fmt.Errorf("空段")
	}
	if len(seg) > MaxIdentifierLength {
		return fmt.Errorf("段长度超过 %d", MaxIdentifierLength)
	}
	if !identifierSegmentPattern.MatchString(seg) {
		return fmt.Errorf("段 %q 含非法字符；仅允许字母数字下划线且首字符非数字", seg)
	}
	return nil
}

// ValidateJSONPathSegment 校验 JSON 路径里单段 key。
// 严格限制为 `^[A-Za-z_][\w]*$`，不允许 `[`、`*`、`'`、`"`、空格、`$`、`.` 等。
func ValidateJSONPathSegment(seg string) error {
	if seg == "" {
		return fmt.Errorf("JSON path 段不能为空")
	}
	if len(seg) > MaxIdentifierLength {
		return fmt.Errorf("JSON path 段长度超过 %d", MaxIdentifierLength)
	}
	if !jsonPathSegmentPattern.MatchString(seg) {
		return fmt.Errorf("JSON path 段 %q 含非法字符", seg)
	}
	return nil
}

// ValidateJSONPath 校验完整 JSON path，多段以 `.` 连接，例如 `status` 或 `payload.user.id`。
func ValidateJSONPath(path string) error {
	if path == "" {
		return fmt.Errorf("JSON path 不能为空")
	}
	for seg := range strings.SplitSeq(path, ".") {
		if err := ValidateJSONPathSegment(seg); err != nil {
			return err
		}
	}
	return nil
}

// ValidateJoinOp Join.On.Op 白名单。
func ValidateJoinOp(op string) error {
	if _, ok := allowedJoinOps[op]; !ok {
		return fmt.Errorf("非法 JOIN 操作符 %q：仅允许 = / <>", op)
	}
	return nil
}
