package constant

const (
	KeyTransfersPrefix    = "web3:transfers"           // Transfer record cache prefix
	KeyFundMePrefix       = "web3:fundme"              // FundMe record cache prefix
	KeyBalancePrefix      = "web3:balance"             // Balance cache prefix
	LockKeyPrefix         = "web3:lock"                // Distributed lock prefix
	KeyLastBlock          = "web3:listener:last_block" // Last processed block cache
	KeyBatchSaveLock      = "web3:batch_save:lock"     // Batch save lock
	KeyTokenInfoPrefix    = "web3:token_info"          // Token info cache prefix
	MaxCacheTransferCount = 50
	TxTypeNative          = "native"
	TxTypeToken           = "token"
	ABITypeERC20          = "erc20"                                      // Standard ERC20
	ABITypeERC721         = "erc721"                                     // Standard ERC721
	ABITypeCustomStable   = "custom_stable"                              // Custom stablecoin
	ABITypeBridge         = "bridge"                                     // Cross-chain bridge
	NativeTokenAddress    = "0x0000000000000000000000000000000000000000" // Native token address
	NativeETHPlaceholder  = "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE" // DeFi-standard native ETH placeholder
)

var ABIMap = map[string]string{
	ABITypeERC20:  `[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`,
	ABITypeERC721: `[{"anonymous":false,"inputs":[{"indexed":true,"name":"from", "type": "address"},{"indexed": true, "name": "to", "type": "address"},{"indexed": true, "name": "tokenId", "type": "uint256"}],"name": "Transfer","type": "event"}]`,
}
