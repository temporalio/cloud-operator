---
description: Unit test conventions for Go test files
globs:
  - "**/*_test.go"
paths:
  - "**/*_test.go"
---

# Unit test conventions

RULE: Write every unit test as a table test, even with a single case. Adding
a case is then one more row, each case runs as a named subtest you can rerun
with `-run`, and every case goes through the same assertions.

```go
cases := []struct {
	name    string
	subject any
	expErr  string
}{
	{
		name:    "fail if subject is an unsupported type",
		subject: int64(1),
		expErr:  "unsupported type int64",
	},
}
for _, tc := range cases {
	t.Run(tc.name, func(tt *testing.T) {
		require := require.New(tt)
		err := manager.New().Register(tc.subject)
		if tc.expErr != "" {
			require.ErrorContains(err, tc.expErr)
			return
		}
		require.Nil(err)
	})
}
```

RULE: Follow the shape above. Prefix expected fields with `exp` and call the
actual value `got`. Name failure cases "fail if ..." and success cases
"<input> yields <result>", with failures first. Set only the fields a case is
about.

RULE: Name tests `TestType_Method` or `TestFunc` and put them in the external
`_test` package. When groups of inputs need different fixtures, split them
into suffixed functions such as
`TestLoader_ClientOptions_ControllerSpec_APIKeyCredentials` instead of one
table with mostly empty fields.

RULE: Build shared fixtures once above the table. Build the code under test
and its fakes inside each subtest. Give each case its own copy of anything the
code may mutate.

RULE: Never reach a real network or cluster. Use the fakes and mocks in
`pkg/testutil` and kube-temporal's `pkg/testutil`, and add a missing one there
instead of in the test file.

RULE: Use `require`, except in callbacks that run on another goroutine, such
as fake server handlers, where `require` can't stop the test. Use `assert`
there.

## Code that calls Temporal Cloud

RULE: Test it against the fake gRPC server in
`pkg/testutil/temporal/cloud/fake/server`, configured per case through a
`serverOpts` field. To simulate a Temporal Cloud error, return
`status.Error` with the gRPC code the code under test checks, such as
`codes.NotFound`. gRPC sends any other error to the client as
`codes.Unknown`, so a plain `fmt.Errorf` never reaches a code-specific
branch.

RULE: Each fake method keeps only the last handler set for it, so
`OnGetNamespaceAssert` replaces an earlier `OnGetNamespaceReturn`. An asserter
that is never called also passes. To check requests, record them in `WithX`
handlers and assert after the call, as `deleteFakeCloud` in
`controller/namespace/delete_test.go` does.

RULE: A fake method without a handler returns `codes.Unimplemented`. Give
every method the code calls a handler, adding it to the fake server if it's
missing, and never write a test that passes on that error.
