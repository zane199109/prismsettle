// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Test} from "forge-std/Test.sol";
import {PrismSettleJob} from "../src/PrismSettleJob.sol";
import {PrismSettleRegistry} from "../src/PrismSettleRegistry.sol";
import {IX402Facilitator} from "../src/interfaces/IX402Facilitator.sol";
import {MockERC20} from "../src/mocks/MockERC20.sol";
import {MockX402Facilitator} from "../src/mocks/MockX402Facilitator.sol";

/// @title X402IntegrationTest
/// @notice Phase 9 task 9.8 — FR-AP06~AP09 integration coverage.
///         Validates the dual funding path (x402 vs ERC-20 fallback)
///         including the facilitator=address(0) disabled case and
///         receipt-replay protection across the fallback boundary.
contract X402IntegrationTest is Test {
    MockERC20 internal token;
    MockX402Facilitator internal facilitator;
    PrismSettleJob internal jobWithFacilitator;
    PrismSettleJob internal jobNoFacilitator; // facilitator=address(0)
    PrismSettleRegistry internal registry;

    address internal buyer = address(0xB0B);
    uint256 internal constant AGENT_ID = 0x2222;
    uint256 internal constant AGENT_PROVIDER = 0x5555;
    uint256 internal constant FUND_AMOUNT = 100 ether;
    uint64 internal constant DEADLINE_OFFSET = 1 hours;

    function setUp() public {
        vm.warp(1 days);

        token = new MockERC20("MockUSDC", "USDC");
        facilitator = new MockX402Facilitator(address(token), FUND_AMOUNT);
        registry = new PrismSettleRegistry();
        registry.registerAgent(AGENT_PROVIDER, '{"endpointUrl":"","capabilities":"provider"}');
        registry.seedAgent(AGENT_PROVIDER, uint96(0.7e18));

        // Two Job instances: one with facilitator, one without (fallback-only).
        jobWithFacilitator = new PrismSettleJob(address(token), address(facilitator), 60);
        jobWithFacilitator.setRegistry(address(registry));
        jobNoFacilitator = new PrismSettleJob(address(token), address(0), 60);
        jobNoFacilitator.setRegistry(address(registry));

        token.mint(buyer, 10_000 ether);
        // Buyer approves both Job contracts + the facilitator.
        vm.startPrank(buyer);
        token.approve(address(jobWithFacilitator), type(uint256).max);
        token.approve(address(jobNoFacilitator), type(uint256).max);
        token.approve(address(facilitator), type(uint256).max);
        vm.stopPrank();
    }

    // ------------------------------------------------------------------
    // FR-AP09: facilitator=address(0) → ERC-20 fallback succeeds
    // ------------------------------------------------------------------

    function testFallbackERC20WhenNoFacilitator() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = jobNoFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        // ERC-20 path must succeed even though facilitator is address(0).
        uint256 balBefore = token.balanceOf(buyer);
        vm.prank(buyer);
        jobNoFacilitator.fundViaToken(jobId, FUND_AMOUNT, "");

        assertEq(token.balanceOf(buyer), balBefore - FUND_AMOUNT);
        assertEq(token.balanceOf(address(jobNoFacilitator)), FUND_AMOUNT);
        (,,, uint256 amount,,,,,,) = jobNoFacilitator.getJobState(jobId);
        assertEq(amount, FUND_AMOUNT);
    }

    // ------------------------------------------------------------------
    // FR-AP09: receipt path reverts when facilitator=address(0)
    // ------------------------------------------------------------------

    function testRevertX402PathWhenNoFacilitator() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = jobNoFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        bytes memory receipt = bytes("x402-receipt-no-facilitator");
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: no facilitator");
        jobNoFacilitator.fundViaToken(jobId, 0, receipt);
    }

    // ------------------------------------------------------------------
    // FR-AP09: same Job contract can switch paths (facilitator present)
    //          — x402 works, ERC-20 also works (different Jobs)
    // ------------------------------------------------------------------

    function testDualPathOnSameContract() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;

        // Job 1: x402 path
        vm.prank(buyer);
        uint256 jobId1 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        jobWithFacilitator.fundViaToken(jobId1, 0, bytes("receipt-1"));

        // Job 2: ERC-20 fallback path on the SAME contract
        vm.prank(buyer);
        uint256 jobId2 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        jobWithFacilitator.fundViaToken(jobId2, FUND_AMOUNT, "");

        // Both Jobs funded correctly.
        (,,, uint256 amt1,,,,,,) = jobWithFacilitator.getJobState(jobId1);
        (,,, uint256 amt2,,,,,,) = jobWithFacilitator.getJobState(jobId2);
        assertEq(amt1, FUND_AMOUNT); // facilitator fixed amount
        assertEq(amt2, FUND_AMOUNT); // explicit ERC-20 amount
        assertEq(token.balanceOf(address(jobWithFacilitator)), 2 * FUND_AMOUNT);
    }

    // ------------------------------------------------------------------
    // FR-AP06: receipt format is opaque — arbitrary non-empty bytes accepted
    // ------------------------------------------------------------------

    function testReceiptIsOpaqueBytes() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;

        // Short receipt
        vm.prank(buyer);
        uint256 jobId1 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        jobWithFacilitator.fundViaToken(jobId1, 0, bytes("a"));

        // Long receipt (256 bytes)
        vm.prank(buyer);
        uint256 jobId2 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        jobWithFacilitator.fundViaToken(jobId2, 0, new bytes(256));

        // Binary receipt (non-UTF8)
        vm.prank(buyer);
        uint256 jobId3 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        bytes memory binaryReceipt = hex"deadbeefcafebabe";
        vm.prank(buyer);
        jobWithFacilitator.fundViaToken(jobId3, 0, binaryReceipt);

        // All three funded
        (,,, uint256 amt1,,,,,,) = jobWithFacilitator.getJobState(jobId1);
        (,,, uint256 amt2,,,,,,) = jobWithFacilitator.getJobState(jobId2);
        (,,, uint256 amt3,,,,,,) = jobWithFacilitator.getJobState(jobId3);
        assertEq(amt1, FUND_AMOUNT);
        assertEq(amt2, FUND_AMOUNT);
        assertEq(amt3, FUND_AMOUNT);
    }

    // ------------------------------------------------------------------
    // FR-AP09: receipt used on x402 path cannot be reused on fallback path
    //          (replay protection is receipt-based, not path-based)
    // ------------------------------------------------------------------

    function testRevertReplayAfterFallback() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        bytes memory receipt = bytes("receipt-replay-test");

        // x402 path consumes the receipt
        vm.prank(buyer);
        uint256 jobId1 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        jobWithFacilitator.fundViaToken(jobId1, 0, receipt);

        // Replaying same receipt must revert (even on a different Job)
        vm.prank(buyer);
        uint256 jobId2 = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));
        vm.prank(buyer);
        vm.expectRevert("PrismSettle: receipt used");
        jobWithFacilitator.fundViaToken(jobId2, 0, receipt);
    }

    // ------------------------------------------------------------------
    // FR-AP09: amount must be 0 when receipt present (x402 path)
    // ------------------------------------------------------------------

    function testRevertX402WithNonZeroAmount() public {
        uint64 deadline = uint64(block.timestamp) + DEADLINE_OFFSET;
        vm.prank(buyer);
        uint256 jobId = jobWithFacilitator.createJob(AGENT_ID, 0, deadline, address(0), 0, address(0));

        vm.prank(buyer);
        vm.expectRevert("PrismSettle: amount must be 0 with receipt");
        jobWithFacilitator.fundViaToken(jobId, FUND_AMOUNT, bytes("receipt"));
    }
}