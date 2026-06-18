// Deploys P2PEscrow with the arbiter set to ARBITER_ADDRESS (the platform
// backend signer). Falls back to the deployer account if ARBITER_ADDRESS unset.
//
//   npx hardhat run scripts/deploy.js --network localhost
//   ARBITER_ADDRESS=0x... npx hardhat run scripts/deploy.js --network sepolia
const hre = require("hardhat");

async function main() {
  const [deployer] = await hre.ethers.getSigners();
  const arbiter = process.env.ARBITER_ADDRESS || deployer.address;

  console.log("Deploying P2PEscrow...");
  console.log("  deployer:", deployer.address);
  console.log("  arbiter :", arbiter);

  const Escrow = await hre.ethers.getContractFactory("P2PEscrow");
  const escrow = await Escrow.deploy(arbiter);
  await escrow.waitForDeployment();

  const address = await escrow.getAddress();
  console.log("\nP2PEscrow deployed at:", address);
  console.log("\nSet this in your backend .env:");
  console.log("  ESCROW_CONTRACT_ADDRESS=" + address);
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
