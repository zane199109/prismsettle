// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

/// @title IX402Facilitator
/// @notice Minimal interface for the x402 payment facilitator.
/// @dev    The real Monad x402 Facilitator verifies a receipt and pulls
///         USDC from the payer to the receiver. This interface captures
///         only the single function PrismSettleJob calls; the Facilitator
///         address is injected at construction and may be address(0) to
///         disable the x402 path entirely (ERC-20 fallback only).
interface IX402Facilitator {
    /// @notice Settle a payment using an x402 receipt.
    /// @param payer    The buyer whose USDC will be pulled.
    /// @param receiver The Job contract that receives the funds.
    /// @param receipt  Opaque x402 receipt bytes (Facilitator-defined format).
    /// @return amount  USDC amount actually transferred.
    function settleWithReceipt(address payer, address receiver, bytes calldata receipt)
        external
        returns (uint256 amount);
}
