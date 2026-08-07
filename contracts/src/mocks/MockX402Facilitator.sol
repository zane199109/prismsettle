// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {IX402Facilitator} from "../interfaces/IX402Facilitator.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/// @title MockX402Facilitator
/// @notice Test stand-in for the Monad x402 Facilitator.
///         The real Facilitator verifies a receipt cryptographically and
///         determines the amount; this mock returns a fixed amount
///         configured at construction and pulls ERC-20 from the payer.
/// @dev    Test-only. The receipt format is treated as opaque: any
///         non-empty bytes are accepted (replay is still blocked by
///         PrismSettleJob.usedReceipts).
contract MockX402Facilitator is IX402Facilitator {
    IERC20 public token;
    uint256 public fixedAmount;

    constructor(address token_, uint256 fixedAmount_) {
        token = IERC20(token_);
        fixedAmount = fixedAmount_;
    }

    function settleWithReceipt(address payer, address receiver, bytes calldata)
        external
        override
        returns (uint256 amount)
    {
        amount = fixedAmount;
        require(token.transferFrom(payer, receiver, amount), "MockX402: transferFrom failed");
    }
}
