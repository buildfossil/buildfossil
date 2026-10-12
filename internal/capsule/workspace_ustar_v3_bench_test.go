package capsule

import "testing"

func BenchmarkValidateWorkspaceUSTARPathV3(b *testing.B) {
	const name = "internal/toml-test/tests/invalid/string/" +
		"basic-multiline-out-of-range-unicode-escape-01.toml"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := validateWorkspaceUSTARPathV3(name); err != nil {
			b.Fatal(err)
		}
	}
}
