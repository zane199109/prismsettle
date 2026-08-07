// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console} from "forge-std/Script.sol";
import {BaselineRegistry} from "../src/bench/BaselineRegistry.sol";

/// @title DeployBaseline
/// @notice Deploy the V0 BaselineRegistry (single-slot) for OCC benchmark.
contract DeployBaseline is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("DEPLOYER_KEY");
        address deployer = vm.addr(deployerKey);

        // Read evaluator address from env (same key as PRISM_EVALUATOR_KEY)
        uint256 evaluatorKey = vm.envUint("PRISM_EVALUATOR_KEY");
        address evaluator = vm.addr(evaluatorKey);

        vm.startBroadcast(deployerKey);

        BaselineRegistry v0 = new BaselineRegistry();

        // Grant REGISTRY_EVALUATOR_ROLE so benchmark can call submitValidation(source=1)
        bytes32 EVAL_ROLE = v0.REGISTRY_EVALUATOR_ROLE();
        v0.grantRole(EVAL_ROLE, evaluator);

        console.log("BaselineRegistry (V0): %s", address(v0));
        console.log("Deployer: %s", deployer);
        console.log("Evaluator: %s", evaluator);

        vm.stopBroadcast();
    }
}
