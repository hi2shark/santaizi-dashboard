package utils

import (
	"testing"
)

type testSt struct {
	input  string
	output string
}

func TestNotification(t *testing.T) {
	cases := []testSt{
		// 纯 IPv4：保留 A.B，隐藏 C.D
		{
			input:  "103.80.236.249",
			output: "103.80.*.*",
		},
		// 纯 IPv6 完整形：保留前 3 个 hextet
		{
			input:  "d5ce:d811:cdb8:067a:a873:2076:9521:9d2d",
			output: "d5ce:d811:cdb8:****::",
		},
		// IPv6 :: 压缩形：展开后取前三组，可能含 0 组
		{
			input:  "d5ce::cdb8:067a:a873:2076:9521:9d2d",
			output: "d5ce:0:cdb8:****::",
		},
		{
			input:  "d5ce::cdb8:067a:a873:2076:9d2d",
			output: "d5ce:0:0:****::",
		},
		// IPv4-mapped IPv6 按 IPv4 逻辑处理
		{
			input:  "::ffff:103.80.236.249",
			output: "103.80.*.*",
		},
		// 双栈串：按 / 逐段打码后拼回
		{
			input:  "103.80.236.249/d5ce:d811:cdb8:067a:a873:2076:9521:9d2d",
			output: "103.80.*.*/d5ce:d811:cdb8:****::",
		},
		// 非法串回退旧正则打码
		{
			input:  "1.2.3",
			output: "1.****.3",
		},
		{
			input:  "1234:5678:9abc:def0:1234:5678:9abc:defg",
			output: "1234:5678:****:9abc:defg",
		},
		// 空串原样返回
		{
			input:  "",
			output: "",
		},
	}

	for _, c := range cases {
		if got := IPDesensitize(c.input); got != c.output {
			t.Errorf("IPDesensitize(%q) = %q, expected %q", c.input, got, c.output)
		}
	}
}

func TestGenerGenerateRandomString(t *testing.T) {
	generatedString := make(map[string]bool)
	for i := 0; i < 100; i++ {
		str, err := GenerateRandomString(32)
		if err != nil {
			t.Fatalf("Error: %s", err)
		}
		if len(str) != 32 {
			t.Fatalf("Expected 32, but got %d", len(str))
		}
		if generatedString[str] {
			t.Fatalf("Duplicated string: %s", str)
		}
		generatedString[str] = true
	}
}
