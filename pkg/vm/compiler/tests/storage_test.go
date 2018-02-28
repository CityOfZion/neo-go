package compiler

var storageTestCases = []testCase{
	{
		"interop storage test",
		`
		package foo

		import "github.com/CityOfZion/neo-go/pkg/vm/smartcontract/storage"

		func Main() int {
			ctx := storage.GetContext()
			storage.Put(ctx, "amount", 1000)
			amount := storage.GetInt(ctx, "amount")
			return amount
		}
		`,
		"54c56b6168164e656f2e53746f726167652e476574436f6e74657874616c766b00527ac46c766b00c306616d6f756e7402e803527261680f4e656f2e53746f726167652e507574616c766b00c306616d6f756e747c61680f4e656f2e53746f726167652e476574616c766b51527ac46203006c766b51c3616c756651c56b62030000616c756654c56b6c766b00527ac46c766b51527ac46c766b52527ac462030000616c756653c56b6c766b00527ac46c766b51527ac462030000616c7566",
	},
}
