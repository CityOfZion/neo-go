package keytestcases

// Ktype represents key testcase values (different encodings of the key).
type Ktype struct {
	Address,
	PrivateKey,
	PublicKey,
	Wif,
	Passphrase,
	EncryptedWif string
	Invalid bool
}

// Arr contains a set of known keys in Ktype format.
var Arr = []Ktype{
	{
		Address:      "NNWAo5vdVJz1oyCuNiaTBA3amBHnWCF4Yk",
		PrivateKey:   "7d128a6d096f0c14c3a25a2b0c41cf79661bfcb4a8cc95aaaea28bde4d732344",
		PublicKey:    "02028a99826edc0c97d18e22b6932373d908d323aa7f92656a77ec26e8861699ef",
		Wif:          "L1QqQJnpBwbsPGAuutuzPTac8piqvbR1HRjrY5qHup48TBCBFe4g",
		Passphrase:   "city of zion",
		EncryptedWif: "6PYSeMMbJtfMRD81eHzriwrRKquu2dgLNurYcAbmJa7YqAiThgA2vGQu5o",
	},
	{
		Address:      "NiwvMyWYeNghLG8tDyKkWwuZV3wS8CPrrV",
		PrivateKey:   "9ab7e154840daca3a2efadaf0df93cd3a5b51768c632f5433f86909d9b994a69",
		PublicKey:    "031d8e1630ce640966967bc6d95223d21f44304133003140c3b52004dc981349c9",
		Wif:          "L2QTooFoDFyRFTxmtiVHt5CfsXfVnexdbENGDkkrrgTTryiLsPMG",
		Passphrase:   "我的密码",
		EncryptedWif: "6PYKWKaq5NMyjt8cjvnJnvmV13inhFuePpWZMkddFAMCgjC3ETt7kX16V9",
	},
	{
		Address:      "NTWHAzB82LRGWNuuqjVyyzpGvF3WxbbPoG",
		PrivateKey:   "3edee7036b8fd9cef91de47386b191dd76db2888a553e7736bb02808932a915b",
		PublicKey:    "02232ce8d2e2063dce0451131851d47421bfc4fc1da4db116fca5302c0756462fa",
		Wif:          "KyKvWLZsNwBJx5j9nurHYRwhYfdQUu9tTEDsLCUHDbYBL8cHxMiG",
		Passphrase:   "MyL33tP@33w0rd",
		EncryptedWif: "6PYSzKoJBQMj9uHUv1Sc2ZhMrydqDF8ZCTeE9FuPiNdEx7Lo9NoEuaXeyk",
	},
	{
		Address:      "xdf4UGKevVrMR1j3UkPsuoYKSC4ocoAkKx",
		PrivateKey:   "zzdee7036b8fd9cef91de47386b191dd76db2888a553e7736bb02808932a915b",
		PublicKey:    "zz232ce8d2e2063dce0451131851d47421bfc4fc1da4db116fca5302c0756462fa",
		Wif:          "zzKvWLZsNwBJx5j9nurHYRwhYfdQUu9tTEDsLCUHDbYBL8cHxMiG",
		Passphrase:   "zzL33tP@33w0rd",
		EncryptedWif: "6PYSzKoJBQMj9uHUv1Sc2ZhMrydqDF8ZCTeE9FuPiNdEx7Lo9NoEuaXeyk",
		Invalid:      true,
	},
}
