// Copyright (c) 2018 CommerceBlock Team
// Use of this source code is governed by an MIT
// license that can be found in the LICENSE file.

package attestation

import (
	"bytes"
	"context"
	"errors"
	"log"
	"sync"
	"time"

	confpkg "mainstay/config"
	"mainstay/models"
	"mainstay/server"

	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

// Attestation Service is the main processes that handles generating
// attestations and maintaining communication to a bitcoin wallet

// Attestation state type
type AttestationState int

// Attestation states
const (
	ASTATE_ERROR              AttestationState = -1
	ASTATE_INIT               AttestationState = 0
	ASTATE_NEXT_COMMITMENT    AttestationState = 1
	ASTATE_NEW_ATTESTATION    AttestationState = 2
	ASTATE_SIGN_ATTESTATION   AttestationState = 3
	ASTATE_PRE_SEND_STORE     AttestationState = 4
	ASTATE_SEND_ATTESTATION   AttestationState = 5
	ASTATE_AWAIT_CONFIRMATION AttestationState = 6
	ASTATE_HANDLE_UNCONFIRMED AttestationState = 7
)

// error / warning consts
const (
	ERROR_UNSPENT_NOT_FOUND = "No valid unspent found"

	WARNING_INVALID_ATIME_NEW_ATTESTATION_ARG    = "Warning - Invalid new attestation time argument"
	WARNING_INVALID_ATIME_HANDLE_UNCONFIRMED_ARG = "Warning - Invalid handle unconfirmed time argument"
)

// waiting time schedules
const (
	// fixed waiting time between states
	ATIME_FIXED = 5 * time.Second

	// waiting time for sigs to arrive from multisig nodes
	ATIME_SIGS = 1 * time.Minute

	// waiting time between attemps to check if an attestation has been confirmed
	ATIME_CONFIRMATION = 15 * time.Minute

	// waiting time between consecutive attestations after one was confirmed
	DEFAULT_ATIME_NEW_ATTESTATION = 60 * time.Minute

	// waiting time until we handle an attestation that has not been confirmed
	// usually by increasing the fee of the previous transcation to speed up confirmation
	DEFAULT_ATIME_HANDLE_UNCONFIRMED = 60 * time.Minute
)

// AttestationService structure
// Encapsulates Attest Client and connectivity
// to a Server for updates and requests
type AttestService struct {
	// context required for safe service cancellation
	ctx context.Context

	// waitgroup required to maintain all goroutines
	wg *sync.WaitGroup

	// service config
	config *confpkg.Config

	// client interface for attestation creation and key tweaking
	attester *AttestClient

	// server connection for querying and/or storing information
	server *server.Server

	// interface to signers to send commitments/transactions and receive signatures
	signer AttestSigner

	// mainstain current attestation state, model and error state
	state       AttestationState
	attestation *models.Attestation
	errorState  error
	isRegtest   bool
}

var (
	atimeNewAttestation    time.Duration // delay between attestations - DEFAULTS to DEFAULT_ATIME_NEW_ATTESTATION
	atimeHandleUnconfirmed time.Duration // delay until handling unconfirmed - DEFAULTS to DEFAULT_ATIME_HANDLE_UNCONFIRMED

	attestDelay time.Duration // handle state delay
	confirmTime time.Time     // handle confirmation timing
)

// NewAttestService returns a pointer to an AttestService instance
// Initiates Attest Client and Attest Server
func NewAttestService(ctx context.Context, wg *sync.WaitGroup, server *server.Server, signer AttestSigner, config *confpkg.Config) *AttestService {
	// Check init txid validity
	_, errInitTx := chainhash.NewHashFromStr(config.InitTx())
	if errInitTx != nil {
		log.Fatalf("Incorrect initial transaction id %s\n", config.InitTx())
	}

	// initiate attestation client
	attester := NewAttestClient(config)

	// initiate timing schedules
	atimeNewAttestation = DEFAULT_ATIME_NEW_ATTESTATION
	if config.TimingConfig().NewAttestationMinutes > 0 {
		atimeNewAttestation = time.Duration(config.TimingConfig().NewAttestationMinutes) * time.Minute
	} else {
		log.Println(WARNING_INVALID_ATIME_NEW_ATTESTATION_ARG)
	}
	log.Printf("Time new attestation set to: %v\n", atimeNewAttestation)
	atimeHandleUnconfirmed = DEFAULT_ATIME_HANDLE_UNCONFIRMED
	if config.TimingConfig().HandleUnconfirmedMinutes > 0 {
		atimeHandleUnconfirmed = time.Duration(config.TimingConfig().HandleUnconfirmedMinutes) * time.Minute
	} else {
		log.Println(WARNING_INVALID_ATIME_HANDLE_UNCONFIRMED_ARG)
	}
	log.Printf("Time handle unconfirmed set to: %v\n", atimeHandleUnconfirmed)

	return &AttestService{ctx, wg, config, attester, server, signer, ASTATE_INIT, models.NewAttestationDefault(), nil, config.Regtest()}
}

// Run Attest Service
func (s *AttestService) Run() {
	defer s.wg.Done()

	attestDelay = 30 * time.Second // add some delay for subscribers to have time to set up

	for { //Doing attestations using attestation client and waiting for transaction confirmation
		timer := time.NewTimer(attestDelay)
		select {
		case <-s.ctx.Done():
			log.Println("Shutting down Attestation Service...")
			return
		case <-timer.C:
			// do next attestation state
			s.doAttestation()

			// for testing - overwrite delay
			if s.isRegtest {
				attestDelay = 10 * time.Second
			}

			log.Printf("********** sleeping for: %s ...\n", attestDelay.String())
		}
	}
}

// ASTATE_ERROR
// - Print error state and re-initiate attestation
func (s *AttestService) doStateError() {
	log.Println("*AttestService* ATTESTATION SERVICE FAILURE")
	log.Println(s.errorState)
	s.state = ASTATE_INIT
}

// part of ASTATE_INIT
// handle case when an unconfirmed transactions is found in the mempool
// fetch attestation information and set service state to ASTATE_AWAIT_CONFIRMATION
func (s *AttestService) stateInitUnconfirmed(unconfirmedTxid chainhash.Hash) {
	commitment, commitmentErr := s.server.GetAttestationCommitment(unconfirmedTxid, false)
	if s.setFailure(commitmentErr) {
		return // will rebound to init
	}
	log.Printf("********** found unconfirmed attestation: %s\n", unconfirmedTxid.String())
	s.attestation = models.NewAttestation(unconfirmedTxid, &commitment) // initialise attestation
	rawTx, _ := s.config.MainClient().GetRawTransaction(&unconfirmedTxid)
	s.attestation.Tx = *rawTx.MsgTx() // set msgTx

	s.state = ASTATE_AWAIT_CONFIRMATION // update attestation state
	confirmTime = time.Now()
}

// part of ASTATE_INIT
// handle case when an unspent transaction is found in the wallet
// if the unspent is a previous attestation, update database info
// initiate a new attestation and inform signers of commitment
func (s *AttestService) stateInitUnspent(unspent btcjson.ListUnspentResult) {
	unspentTxid, _ := chainhash.NewHashFromStr(unspent.TxID)
	commitment, commitmentErr := s.server.GetAttestationCommitment(*unspentTxid)
	if s.setFailure(commitmentErr) {
		return // will rebound to init
	} else if (commitment.GetCommitmentHash() != chainhash.Hash{}) {
		log.Printf("********** found confirmed attestation: %s\n", unspentTxid.String())
		s.attestation = models.NewAttestation(*unspentTxid, &commitment)
		// update server with latest confirmed attestation
		s.attestation.Confirmed = true
		rawTx, _ := s.config.MainClient().GetRawTransaction(unspentTxid)
		walletTx, _ := s.config.MainClient().GetTransaction(unspentTxid)
		s.attestation.Tx = *rawTx.MsgTx()  // set msgTx
		s.attestation.UpdateInfo(walletTx) // set tx info

		errUpdate := s.server.UpdateLatestAttestation(*s.attestation)
		if s.setFailure(errUpdate) {
			return // will rebound to init
		}
	} else {
		log.Println("********** found unspent transaction, initiating staychain")
		s.attestation = models.NewAttestationDefault()
	}
	confirmedHash := s.attestation.CommitmentHash()
	s.signer.SendConfirmedHash((&confirmedHash).CloneBytes()) // update clients

	s.state = ASTATE_NEXT_COMMITMENT // update attestation state
}

// part of ASTATE_INIT
// handles wallet failure when neither unconfirmed or unspent is found
// above case should never actually happen - untested grey area
// TODO: sort this state, as implementation below is incorrect
func (s *AttestService) stateInitWalletFailure() {

	log.Println("********** wallet failure")
	s.state = ASTATE_INIT

	// // no unspent so there must be a transaction waiting confirmation not on the mempool
	// // check server for latest unconfirmed attestation
	// lastCommitmentHash, latestErr := s.server.GetLatestAttestationCommitmentHash(false)
	// if s.setFailure(latestErr) {
	//     return // will rebound to init
	// }
	// commitment, commitmentErr := s.server.GetAttestationCommitment(lastCommitmentHash, false)
	// if s.setFailure(commitmentErr) {
	//     return // will rebound to init
	// }
	// log.Printf("********** found unconfirmed attestation: %s\n", lastCommitmentHash.String())
	// s.attestation = models.NewAttestation(lastCommitmentHash, &commitment) // initialise attestation
	// rawTx, _ := s.config.MainClient().GetRawTransaction(&unconfirmedTxid)
	// s.attestation.Tx = *rawTx.MsgTx() // set msgTx

	// s.state = ASTATE_AWAIT_CONFIRMATION // update attestation state
	// confirmTime = time.Now()

}

// ASTATE_INIT
// - Check if there are unconfirmed or unspent transactions in the client
// - Update server with latest attestation information
// - If no transaction found wait, else initiate new attestation
// - If no attestation found, check last unconfirmed from db
func (s *AttestService) doStateInit() {
	log.Println("*AttestService* INITIATING ATTESTATION PROCESS")

	// find the state of the attestation
	unconfirmed, unconfirmedTxid, unconfirmedErr := s.attester.getUnconfirmedTx()
	if s.setFailure(unconfirmedErr) {
		return // will rebound to init
	} else if unconfirmed { // check mempool for unconfirmed - added check in case something gets rejected
		// handle init unconfirmed case
		s.stateInitUnconfirmed(unconfirmedTxid)
	} else {
		success, unspent, unspentErr := s.attester.findLastUnspent()
		if s.setFailure(unspentErr) {
			return // will rebound to init
		} else if success {
			// handle init unspent case
			s.stateInitUnspent(unspent)
		} else {
			// handle wallet failure case
			s.stateInitWalletFailure()
		}
	}
}

// ASTATE_NEXT_COMMITMENT
// - Get latest commitment from server
// - Check if commitment has already been attested
// - Send commitment to client signers
// - Initialise new attestation
func (s *AttestService) doStateNextCommitment() {
	log.Println("*AttestService* NEW ATTESTATION COMMITMENT")

	// get latest commitment hash from server
	latestCommitment, latestErr := s.server.GetClientCommitment()
	if s.setFailure(latestErr) {
		return // will rebound to init
	}
	latestCommitmentHash := latestCommitment.GetCommitmentHash()

	// check if commitment has already been attested
	log.Printf("********** received commitment hash: %s\n", latestCommitmentHash.String())
	if latestCommitmentHash == s.attestation.CommitmentHash() {
		log.Printf("********** Skipping attestation - Client commitment already attested")
		attestDelay = atimeNewAttestation // sleep
		return                            // will remain at the same state
	}

	// publish new commitment hash to clients
	s.signer.SendNewHash((&latestCommitmentHash).CloneBytes())

	// initialise new attestation with commitment
	s.attestation = models.NewAttestationDefault()
	s.attestation.SetCommitment(&latestCommitment)

	s.state = ASTATE_NEW_ATTESTATION // update attestation state
}

// ASTATE_NEW_ATTESTATION
// - Generate new pay to address for attestation transaction using client commitment
// - Create new unsigned transaction using the last unspent
// - If a topup unspent exists, add this to the new attestation
// - Publish unsigned transaction to signer clients
// - add ATIME_SIGS waiting time
func (s *AttestService) doStateNewAttestation() {
	log.Println("*AttestService* NEW ATTESTATION")

	// Get key and address for next attestation using client commitment
	key, keyErr := s.attester.GetNextAttestationKey(s.attestation.CommitmentHash())
	if s.setFailure(keyErr) {
		return // will rebound to init
	}
	paytoaddr, _ := s.attester.GetNextAttestationAddr(key, s.attestation.CommitmentHash())
	log.Printf("********** importing pay-to addr: %s ...\n", paytoaddr.String())
	importErr := s.attester.ImportAttestationAddr(paytoaddr)
	if s.setFailure(importErr) {
		return // will rebound to init
	}

	// Generate new unsigned attestation transaction from last unspent
	success, unspent, unspentErr := s.attester.findLastUnspent()
	if s.setFailure(unspentErr) {
		return // will rebound to init
	} else if success {
		var unspentList []btcjson.ListUnspentResult
		unspentList = append(unspentList, unspent)

		// search for topup unspent and add if it exists
		topupFound, topupUnspent, topupUnspentErr := s.attester.findTopupUnspent()
		if s.setFailure(topupUnspentErr) {
			return // will rebound to init
		} else if topupFound {
			unspentList = append(unspentList, topupUnspent)
		}

		// create attestation transaction for the list of unspents paying to addr generated
		newTx, createErr := s.attester.createAttestation(paytoaddr, unspentList)
		if s.setFailure(createErr) {
			return // will rebound to init
		}

		s.attestation.Tx = *newTx
		log.Printf("********** pre-sign txid: %s\n", s.attestation.Tx.TxHash().String())

		// publish pre signed transaction
		var txbytes bytes.Buffer
		s.attestation.Tx.Serialize(&txbytes)
		s.signer.SendNewTx(txbytes.Bytes())

		s.state = ASTATE_SIGN_ATTESTATION // update attestation state
		attestDelay = ATIME_SIGS          // add sigs waiting time
	} else {
		s.setFailure(errors.New(ERROR_UNSPENT_NOT_FOUND))
		return // will rebound to init
	}
}

// ASTATE_SIGN_ATTESTATION
// - Collect signatures from client signers
// - Combine signatures them and sign the attestation transaction
func (s *AttestService) doStateSignAttestation() {
	log.Println("*AttestService* SIGN ATTESTATION")

	// Read sigs using subscribers
	sigs := s.signer.GetSigs()
	for sigForInput, _ := range sigs {
		log.Printf("********** received %d signatures for input %d \n",
			len(sigs[sigForInput]), sigForInput)
	}

	// get last confirmed commitment from server
	lastCommitmentHash, latestErr := s.server.GetLatestAttestationCommitmentHash()
	if s.setFailure(latestErr) {
		return // will rebound to init
	}

	// sign attestation with combined sigs and last commitment
	signedTx, signErr := s.attester.signAttestation(&s.attestation.Tx, sigs, lastCommitmentHash)
	if s.setFailure(signErr) {
		return // will rebound to init
	}
	s.attestation.Tx = *signedTx
	s.attestation.Txid = s.attestation.Tx.TxHash()

	s.state = ASTATE_PRE_SEND_STORE // update attestation state
}

// ASTATE_PRE_SEND_STORE
// - Store unconfirmed attestation to server prior to sending
func (s *AttestService) doStatePreSendStore() {
	log.Println("*AttestService* PRE SEND STORE")

	// update server with latest unconfirmed attestation, in case the service fails
	errUpdate := s.server.UpdateLatestAttestation(*s.attestation)
	if s.setFailure(errUpdate) {
		return // will rebound to init
	}

	s.state = ASTATE_SEND_ATTESTATION // update attestation state
}

// ASTATE_SEND_ATTESTATION
// - Send attestation transaction through the client to the network
// - add ATIME_CONFIRMATION waiting time
// - start time for confirmation time
func (s *AttestService) doStateSendAttestation() {
	log.Println("*AttestService* SEND ATTESTATION")

	// sign attestation with combined signatures and send through client to network
	txid, attestationErr := s.attester.sendAttestation(&s.attestation.Tx)
	if s.setFailure(attestationErr) {
		return // will rebound to init
	}
	s.attestation.Txid = txid
	log.Printf("********** attestation transaction committed with txid: (%s)\n", txid)

	s.state = ASTATE_AWAIT_CONFIRMATION // update attestation state
	attestDelay = ATIME_CONFIRMATION    // add confirmation waiting time
	confirmTime = time.Now()            // set time for awaiting confirmation
}

// ASTATE_AWAIT_CONFIRMATION
// - Check if the attestation transaction has been confirmed in the main network
// - If confirmed, initiate new attestation, update server and signer clients
// - Check if ATIME_HANDLE_UNCONFIRMED has elapsed since attestation was sent
// - add ATIME_NEW_ATTESTATION if confirmed or ATIME_CONFIRMATION if not to waiting time
func (s *AttestService) doStateAwaitConfirmation() {
	log.Printf("*AttestService* AWAITING CONFIRMATION \ntxid: (%s)\ncommitment: (%s)\n", s.attestation.Txid.String(), s.attestation.CommitmentHash().String())

	// if attestation has been unconfirmed for too long
	// set to handle unconfirmed state
	if time.Since(confirmTime) > atimeHandleUnconfirmed {
		s.state = ASTATE_HANDLE_UNCONFIRMED
		return
	}

	newTx, err := s.config.MainClient().GetTransaction(&s.attestation.Txid)
	if s.setFailure(err) {
		return // will rebound to init
	}

	if newTx.BlockHash != "" {
		log.Printf("********** attestation confirmed with txid: (%s)\n", s.attestation.Txid.String())

		// update server with latest confirmed attestation
		s.attestation.Confirmed = true
		s.attestation.UpdateInfo(newTx)
		errUpdate := s.server.UpdateLatestAttestation(*s.attestation)
		if s.setFailure(errUpdate) {
			return // will rebound to init
		}

		s.attester.Fees.ResetFee(s.isRegtest) // reset client fees

		confirmedHash := s.attestation.CommitmentHash()
		s.signer.SendConfirmedHash((&confirmedHash).CloneBytes()) // update clients

		s.state = ASTATE_NEXT_COMMITMENT                            // update attestation state
		attestDelay = atimeNewAttestation - time.Since(confirmTime) // add new attestation waiting time - subtract waiting time
	} else {
		attestDelay = ATIME_CONFIRMATION // add confirmation waiting time
	}
}

// ASTATE_HANDLE_UNCONFIRMED
// - Handle attestations that have been unconfirmed for too long
// - Bump attestation fees and re-initiate sign and send process
func (s *AttestService) doStateHandleUnconfirmed() {
	log.Println("*AttestService* HANDLE UNCONFIRMED")

	log.Printf("********** bumping fees for attestation txid: %s\n", s.attestation.Tx.TxHash().String())
	currentTx := &s.attestation.Tx
	bumpErr := s.attester.bumpAttestationFees(currentTx)
	if s.setFailure(bumpErr) {
		return // will rebound to init
	}

	s.attestation.Tx = *currentTx
	log.Printf("********** new pre-sign txid: %s\n", s.attestation.Tx.TxHash().String())

	// re-publish pre signed transaction
	var txbytes bytes.Buffer
	s.attestation.Tx.Serialize(&txbytes)
	s.signer.SendNewTx(txbytes.Bytes())

	s.state = ASTATE_SIGN_ATTESTATION // update attestation state
	attestDelay = ATIME_SIGS          // add sigs waiting time
}

//Main attestation service method - cycles through AttestationStates
func (s *AttestService) doAttestation() {

	// fixed waiting time between states specific states might
	// re-write this to set specific waiting times
	attestDelay = ATIME_FIXED

	switch s.state {

	case ASTATE_ERROR:
		s.doStateError()

	case ASTATE_INIT:
		s.doStateInit()

	case ASTATE_NEXT_COMMITMENT:
		s.doStateNextCommitment()

	case ASTATE_NEW_ATTESTATION:
		s.doStateNewAttestation()

	case ASTATE_SIGN_ATTESTATION:
		s.doStateSignAttestation()

	case ASTATE_PRE_SEND_STORE:
		s.doStatePreSendStore()

	case ASTATE_SEND_ATTESTATION:
		s.doStateSendAttestation()

	case ASTATE_AWAIT_CONFIRMATION:
		s.doStateAwaitConfirmation()

	case ASTATE_HANDLE_UNCONFIRMED:
		s.doStateHandleUnconfirmed()
	}
}

// Check if there is an error and set error state
func (s *AttestService) setFailure(err error) bool {
	if err != nil {
		s.errorState = err
		s.state = ASTATE_ERROR
		return true
	}
	return false
}
