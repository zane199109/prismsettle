// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";

/// @title MockERC20
/// @notice Minimal ERC20 for anvil/testnet USDC stand-in. Production
///         uses the real Monad testnet USDC; this mock is test-only.
contract MockERC20 is ERC20 {
    constructor(string memory name, string memory symbol) ERC20(name, symbol) {}

    /// @notice Mint tokens to any caller for testing. Test-only.
    function mint(address to, uint256 amount) external {
        _mint(to, amount);
    }
}
