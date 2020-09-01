package interopnames

// Names of all used interops.
const (
	SystemBinaryBase58Decode                 = "System.Binary.Base58Decode"
	SystemBinaryBase58Encode                 = "System.Binary.Base58Encode"
	SystemBinaryBase64Decode                 = "System.Binary.Base64Decode"
	SystemBinaryBase64Encode                 = "System.Binary.Base64Encode"
	SystemBinaryDeserialize                  = "System.Binary.Deserialize"
	SystemBinarySerialize                    = "System.Binary.Serialize"
	SystemBlockchainGetBlock                 = "System.Blockchain.GetBlock"
	SystemBlockchainGetContract              = "System.Blockchain.GetContract"
	SystemBlockchainGetHeight                = "System.Blockchain.GetHeight"
	SystemBlockchainGetTransaction           = "System.Blockchain.GetTransaction"
	SystemBlockchainGetTransactionFromBlock  = "System.Blockchain.GetTransactionFromBlock"
	SystemBlockchainGetTransactionHeight     = "System.Blockchain.GetTransactionHeight"
	SystemCallbackCreate                     = "System.Callback.Create"
	SystemCallbackCreateFromMethod           = "System.Callback.CreateFromMethod"
	SystemCallbackCreateFromSyscall          = "System.Callback.CreateFromSyscall"
	SystemCallbackInvoke                     = "System.Callback.Invoke"
	SystemContractCall                       = "System.Contract.Call"
	SystemContractCallEx                     = "System.Contract.CallEx"
	SystemContractCreate                     = "System.Contract.Create"
	SystemContractCreateStandardAccount      = "System.Contract.CreateStandardAccount"
	SystemContractDestroy                    = "System.Contract.Destroy"
	SystemContractIsStandard                 = "System.Contract.IsStandard"
	SystemContractGetCallFlags               = "System.Contract.GetCallFlags"
	SystemContractUpdate                     = "System.Contract.Update"
	SystemEnumeratorConcat                   = "System.Enumerator.Concat"
	SystemEnumeratorCreate                   = "System.Enumerator.Create"
	SystemEnumeratorNext                     = "System.Enumerator.Next"
	SystemEnumeratorValue                    = "System.Enumerator.Value"
	SystemIteratorConcat                     = "System.Iterator.Concat"
	SystemIteratorCreate                     = "System.Iterator.Create"
	SystemIteratorKey                        = "System.Iterator.Key"
	SystemIteratorKeys                       = "System.Iterator.Keys"
	SystemIteratorValues                     = "System.Iterator.Values"
	SystemJSONDeserialize                    = "System.Json.Deserialize"
	SystemJSONSerialize                      = "System.Json.Serialize"
	SystemRuntimeCheckWitness                = "System.Runtime.CheckWitness"
	SystemRuntimeGasLeft                     = "System.Runtime.GasLeft"
	SystemRuntimeGetCallingScriptHash        = "System.Runtime.GetCallingScriptHash"
	SystemRuntimeGetEntryScriptHash          = "System.Runtime.GetEntryScriptHash"
	SystemRuntimeGetExecutingScriptHash      = "System.Runtime.GetExecutingScriptHash"
	SystemRuntimeGetInvocationCounter        = "System.Runtime.GetInvocationCounter"
	SystemRuntimeGetNotifications            = "System.Runtime.GetNotifications"
	SystemRuntimeGetScriptContainer          = "System.Runtime.GetScriptContainer"
	SystemRuntimeGetTime                     = "System.Runtime.GetTime"
	SystemRuntimeGetTrigger                  = "System.Runtime.GetTrigger"
	SystemRuntimeLog                         = "System.Runtime.Log"
	SystemRuntimeNotify                      = "System.Runtime.Notify"
	SystemRuntimePlatform                    = "System.Runtime.Platform"
	SystemStorageDelete                      = "System.Storage.Delete"
	SystemStorageFind                        = "System.Storage.Find"
	SystemStorageGet                         = "System.Storage.Get"
	SystemStorageGetContext                  = "System.Storage.GetContext"
	SystemStorageGetReadOnlyContext          = "System.Storage.GetReadOnlyContext"
	SystemStoragePut                         = "System.Storage.Put"
	SystemStoragePutEx                       = "System.Storage.PutEx"
	SystemStorageAsReadOnly                  = "System.Storage.AsReadOnly"
	NeoCryptoVerifyWithECDsaSecp256r1        = "Neo.Crypto.VerifyWithECDsaSecp256r1"
	NeoCryptoVerifyWithECDsaSecp256k1        = "Neo.Crypto.VerifyWithECDsaSecp256k1"
	NeoCryptoCheckMultisigWithECDsaSecp256r1 = "Neo.Crypto.CheckMultisigWithECDsaSecp256r1"
	NeoCryptoCheckMultisigWithECDsaSecp256k1 = "Neo.Crypto.CheckMultisigWithECDsaSecp256k1"
	NeoCryptoSHA256                          = "Neo.Crypto.SHA256"
	NeoCryptoRIPEMD160                       = "Neo.Crypto.RIPEMD160"
	NeoNativeCall                            = "Neo.Native.Call"
	NeoNativeDeploy                          = "Neo.Native.Deploy"
)

var names = []string{
	SystemBinaryBase58Decode,
	SystemBinaryBase58Encode,
	SystemBinaryBase64Decode,
	SystemBinaryBase64Encode,
	SystemBinaryDeserialize,
	SystemBinarySerialize,
	SystemBlockchainGetBlock,
	SystemBlockchainGetContract,
	SystemBlockchainGetHeight,
	SystemBlockchainGetTransaction,
	SystemBlockchainGetTransactionFromBlock,
	SystemBlockchainGetTransactionHeight,
	SystemCallbackCreate,
	SystemCallbackCreateFromMethod,
	SystemCallbackCreateFromSyscall,
	SystemCallbackInvoke,
	SystemContractCall,
	SystemContractCallEx,
	SystemContractCreate,
	SystemContractCreateStandardAccount,
	SystemContractDestroy,
	SystemContractIsStandard,
	SystemContractGetCallFlags,
	SystemContractUpdate,
	SystemEnumeratorConcat,
	SystemEnumeratorCreate,
	SystemEnumeratorNext,
	SystemEnumeratorValue,
	SystemIteratorConcat,
	SystemIteratorCreate,
	SystemIteratorKey,
	SystemIteratorKeys,
	SystemIteratorValues,
	SystemJSONDeserialize,
	SystemJSONSerialize,
	SystemRuntimeCheckWitness,
	SystemRuntimeGasLeft,
	SystemRuntimeGetCallingScriptHash,
	SystemRuntimeGetEntryScriptHash,
	SystemRuntimeGetExecutingScriptHash,
	SystemRuntimeGetInvocationCounter,
	SystemRuntimeGetNotifications,
	SystemRuntimeGetScriptContainer,
	SystemRuntimeGetTime,
	SystemRuntimeGetTrigger,
	SystemRuntimeLog,
	SystemRuntimeNotify,
	SystemRuntimePlatform,
	SystemStorageDelete,
	SystemStorageFind,
	SystemStorageGet,
	SystemStorageGetContext,
	SystemStorageGetReadOnlyContext,
	SystemStoragePut,
	SystemStoragePutEx,
	SystemStorageAsReadOnly,
	NeoCryptoVerifyWithECDsaSecp256r1,
	NeoCryptoVerifyWithECDsaSecp256k1,
	NeoCryptoCheckMultisigWithECDsaSecp256r1,
	NeoCryptoCheckMultisigWithECDsaSecp256k1,
	NeoCryptoSHA256,
	NeoCryptoRIPEMD160,
	NeoNativeCall,
	NeoNativeDeploy,
}
