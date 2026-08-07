-- Migration: Unify chain_name to "monad_testnet" for all PrismSettle records
--
-- Previously, three separate chain_name values were used:
--   "prismsettle_registry"  (Registry contract events)
--   "prismsettle_job"       (Job contract events)
--   "prismsettle_hook"      (ArbitrationHook contract events)
--
-- Now all three listeners use chain_name = "monad_testnet", with uniqueness
-- guaranteed by the (chain_name, contract_addr) pair.
--
-- Run: psql -h <host> -p <port> -U <user> -d <dbname> -f 001_unify_chain_name.sql

BEGIN;

-- 1. Update chain_events
UPDATE chain_events
SET chain_name = 'monad_testnet'
WHERE chain_name IN ('prismsettle_registry', 'prismsettle_job', 'prismsettle_hook');

-- 2. Update chain_block_states (sync state tracking)
UPDATE chain_block_states
SET chain_name = 'monad_testnet'
WHERE chain_name IN ('prismsettle_registry', 'prismsettle_job', 'prismsettle_hook');

-- 3. Update agent_registry
UPDATE agent_registry
SET chain_name = 'monad_testnet'
WHERE chain_name IN ('prismsettle_registry', 'prismsettle_job', 'prismsettle_hook');

-- 4. Update reorg_events
UPDATE reorg_events
SET chain_name = 'monad_testnet'
WHERE chain_name IN ('prismsettle_registry', 'prismsettle_job', 'prismsettle_hook');

COMMIT;