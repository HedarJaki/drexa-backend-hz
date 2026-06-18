// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title P2PEscrow
/// @notice Native-ETH escrow for Drexa's custodial P2P crypto<->fiat marketplace.
/// @dev The platform backend is the sole operator (the `arbiter`). It funds an
///      escrow on behalf of a seller, then — once the buyer's off-chain fiat
///      payment is confirmed — releases the locked ETH to the buyer, or refunds
///      the seller on cancel / expiry / dispute. `buyer` and `seller` are payout
///      addresses; they never need to hold keys or sign, matching the custodial
///      model where the platform manages all on-chain actions.
contract P2PEscrow {
    enum State {
        None, // 0 - no escrow for this orderId
        Funded, // 1 - seller's crypto locked in the contract
        Paid, // 2 - buyer's fiat payment marked (informational)
        Released, // 3 - crypto released to buyer (terminal)
        Refunded, // 4 - crypto returned to seller (terminal)
        Disputed // 5 - awaiting arbiter resolution
    }

    struct Escrow {
        address payable buyer;
        address payable seller;
        uint256 amount;
        State state;
        uint64 createdAt;
    }

    /// @notice The only account allowed to operate escrows (the platform backend signer).
    address public immutable arbiter;

    mapping(bytes32 => Escrow) private escrows;

    event EscrowCreated(bytes32 indexed orderId, address indexed buyer, address indexed seller, uint256 amount);
    event PaymentMarked(bytes32 indexed orderId);
    event Released(bytes32 indexed orderId, address indexed buyer, uint256 amount);
    event Refunded(bytes32 indexed orderId, address indexed seller, uint256 amount);
    event Disputed(bytes32 indexed orderId);
    event Resolved(bytes32 indexed orderId, bool toBuyer, uint256 amount);

    modifier onlyArbiter() {
        require(msg.sender == arbiter, "P2PEscrow: not arbiter");
        _;
    }

    constructor(address _arbiter) {
        require(_arbiter != address(0), "P2PEscrow: arbiter required");
        arbiter = _arbiter;
    }

    /// @notice Lock `msg.value` ETH into a new escrow keyed by `orderId`.
    /// @dev Called by the backend with the seller's locked crypto value attached.
    function createEscrow(bytes32 orderId, address payable buyer, address payable seller)
        external
        payable
        onlyArbiter
    {
        require(escrows[orderId].state == State.None, "P2PEscrow: escrow exists");
        require(msg.value > 0, "P2PEscrow: amount required");
        require(buyer != address(0) && seller != address(0), "P2PEscrow: bad address");

        escrows[orderId] = Escrow({
            buyer: buyer,
            seller: seller,
            amount: msg.value,
            state: State.Funded,
            createdAt: uint64(block.timestamp)
        });

        emit EscrowCreated(orderId, buyer, seller, msg.value);
    }

    /// @notice Record that the buyer's off-chain fiat payment has been marked paid.
    function markPaid(bytes32 orderId) external onlyArbiter {
        Escrow storage e = escrows[orderId];
        require(e.state == State.Funded, "P2PEscrow: not funded");
        e.state = State.Paid;
        emit PaymentMarked(orderId);
    }

    /// @notice Release escrowed ETH to the buyer (seller confirmed fiat receipt).
    function release(bytes32 orderId) external onlyArbiter {
        Escrow storage e = escrows[orderId];
        require(e.state == State.Funded || e.state == State.Paid, "P2PEscrow: not releasable");
        e.state = State.Released;
        uint256 amt = e.amount;
        e.amount = 0;
        (bool ok, ) = e.buyer.call{value: amt}("");
        require(ok, "P2PEscrow: transfer failed");
        emit Released(orderId, e.buyer, amt);
    }

    /// @notice Refund escrowed ETH to the seller (cancel / expiry / dispute refund).
    function refund(bytes32 orderId) external onlyArbiter {
        Escrow storage e = escrows[orderId];
        require(
            e.state == State.Funded || e.state == State.Paid || e.state == State.Disputed,
            "P2PEscrow: not refundable"
        );
        e.state = State.Refunded;
        uint256 amt = e.amount;
        e.amount = 0;
        (bool ok, ) = e.seller.call{value: amt}("");
        require(ok, "P2PEscrow: transfer failed");
        emit Refunded(orderId, e.seller, amt);
    }

    /// @notice Flag an escrow as disputed, freezing it until the arbiter resolves.
    function raiseDispute(bytes32 orderId) external onlyArbiter {
        Escrow storage e = escrows[orderId];
        require(e.state == State.Funded || e.state == State.Paid, "P2PEscrow: not disputable");
        e.state = State.Disputed;
        emit Disputed(orderId);
    }

    /// @notice Resolve a dispute: send the escrow to the buyer or back to the seller.
    function resolveDispute(bytes32 orderId, bool toBuyer) external onlyArbiter {
        Escrow storage e = escrows[orderId];
        require(e.state == State.Disputed, "P2PEscrow: not disputed");
        uint256 amt = e.amount;
        e.amount = 0;
        if (toBuyer) {
            e.state = State.Released;
            (bool ok, ) = e.buyer.call{value: amt}("");
            require(ok, "P2PEscrow: transfer failed");
        } else {
            e.state = State.Refunded;
            (bool ok, ) = e.seller.call{value: amt}("");
            require(ok, "P2PEscrow: transfer failed");
        }
        emit Resolved(orderId, toBuyer, amt);
    }

    /// @notice Read the current state of an escrow.
    function getEscrow(bytes32 orderId)
        external
        view
        returns (address buyer, address seller, uint256 amount, State state, uint64 createdAt)
    {
        Escrow storage e = escrows[orderId];
        return (e.buyer, e.seller, e.amount, e.state, e.createdAt);
    }
}
