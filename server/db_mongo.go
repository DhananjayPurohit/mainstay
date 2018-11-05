package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"mainstay/models"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/mongodb/mongo-go-driver/bson"
	_ "github.com/mongodb/mongo-go-driver/bson/objectid"
	"github.com/mongodb/mongo-go-driver/mongo"
	"github.com/mongodb/mongo-go-driver/mongo/findopt"
)

const (
    COL_NAME_ATTESTATION = "Attestation"
    COL_NAME_COMMITMENT = "MerkleCommitment"
    COL_NAME_PROOF = "MerkleProof"
    COL_NAME_LATEST_COMMITMENT = "LatestCommitment"
)

// Method to connect to mongo database through config
func dbConnect(ctx context.Context) (*mongo.Database, error) {
	// get this from config
	uri := fmt.Sprintf(`mongodb://%s:%s@%s:%s/%s`,
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME_MAINSTAY"),
	)

	client, err := mongo.NewClient(uri)
	if err != nil {
		return nil, err
	}

	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return client.Database(os.Getenv("DB_NAME_MAINSTAY")), nil
}

// DbMongo struct
type DbMongo struct {
	ctx context.Context
	db  *mongo.Database
}

// Return new DbMongo instance
func NewDbMongo(ctx context.Context) (DbMongo, error) {
	db, errConnect := dbConnect(ctx)

	if errConnect != nil {
		return DbMongo{}, errConnect
	}

	return DbMongo{ctx, db}, nil
}

// Save latest attestation to the database. If attestation already exists then update
func (d *DbMongo) saveAttestation(attestation models.Attestation, confirmed bool) error {

	// new attestation based on Attestation model
	newAttestation := bson.NewDocument(
		bson.EC.SubDocumentFromElements("$set", bson.EC.String("txid", attestation.Txid.String())),
		bson.EC.SubDocumentFromElements("$set", bson.EC.String("merkle_root", attestation.CommitmentHash().String())),
		bson.EC.SubDocumentFromElements("$set", bson.EC.DateTime("inserted_at", int64(time.Now().Unix())*1000)),
		bson.EC.SubDocumentFromElements("$set", bson.EC.Boolean("confirmed", confirmed)),
	)

	// search if attestation already exists
	filterAttestation := bson.NewDocument(
		bson.EC.String("txid", attestation.Txid.String()),
		bson.EC.String("merkle_root", attestation.CommitmentHash().String()),
	)

	// insert or update
	t := bson.NewDocument()
	res := d.db.Collection(COL_NAME_ATTESTATION).FindOneAndUpdate(d.ctx, filterAttestation, newAttestation, findopt.Upsert(true))
	resErr := res.Decode(t)
	if resErr != nil && resErr != mongo.ErrNoDocuments {
		fmt.Printf("couldn't be created: %v\n", resErr)
		return resErr
	}

	return nil
}

// Save attestation commitment to the database
func (d *DbMongo) saveCommitment(commitment models.Commitment) error {
	merkleRoot := commitment.GetCommitmentHash()
	merkleCommitments := commitment.GetMerkleCommitments()
	errSave := d.saveMerkleCommitments(merkleRoot, merkleCommitments)
	if errSave != nil {
		return errSave
	}

	merkleProofs := commitment.GetMerkleProofs()
	errSave = d.saveMerkleProofs(merkleRoot, merkleProofs)
	if errSave != nil {
		return errSave
	}

	return nil
}

// Save merkle commitments
func (d *DbMongo) saveMerkleCommitments(merkleRoot chainhash.Hash, commitments []chainhash.Hash) error {
	for pos := range commitments {
		// new attestation based on Attestation model
		newMerkleCommitment := bson.NewDocument(
			bson.EC.SubDocumentFromElements("$set", bson.EC.String("merkle_root", merkleRoot.String())),
			bson.EC.SubDocumentFromElements("$set", bson.EC.Int32("client_position", int32(pos))),
			bson.EC.SubDocumentFromElements("$set", bson.EC.String("commitment", commitments[pos].String())),
		)

		filterMerkleCommitment := bson.NewDocument(
			bson.EC.String("merkle_root", merkleRoot.String()),
			bson.EC.Int32("client_position", int32(pos)),
		)

		// insert or update
		t := bson.NewDocument()
		res := d.db.Collection("MerkleCommitment").FindOneAndUpdate(d.ctx, filterMerkleCommitment, newMerkleCommitment, findopt.Upsert(true))
		resErr := res.Decode(t)
		if resErr != nil && resErr != mongo.ErrNoDocuments {
			fmt.Printf("couldn't be created: %v\n", resErr)
			return resErr
		}
	}
	return nil
}

// Save merkle proofs
func (d *DbMongo) saveMerkleProofs(merkleRoot chainhash.Hash, proofs []models.CommitmentMerkleProof) error {
	for pos := range proofs {

		el, marshalErr := bson.Marshal(proofs[pos])
		if marshalErr != nil {
			return marshalErr
		}
		out, docErr := bson.ReadDocument(el)
		if docErr != nil {
			return docErr
		}

		newMerkleProof := bson.NewDocument(
			bson.EC.SubDocumentFromElements("$set", bson.EC.String("merkle_root", merkleRoot.String())),
			bson.EC.SubDocumentFromElements("$set", bson.EC.Int32("client_position", int32(pos))),
			bson.EC.SubDocumentFromElements("$set", bson.EC.Interface("proof", out)),
		)

		filterMerkleProof := bson.NewDocument(
			bson.EC.String("merkle_root", merkleRoot.String()),
			bson.EC.Int32("client_position", int32(pos)),
		)

		t := bson.NewDocument()
		res := d.db.Collection("MerkleProof").FindOneAndUpdate(d.ctx, filterMerkleProof, newMerkleProof, findopt.Upsert(true))
		resErr := res.Decode(t)
		if resErr != nil && resErr != mongo.ErrNoDocuments {
			fmt.Printf("couldn't be created: %v\n", resErr)
			return resErr
		}
	}
	return nil
}

// Return latest from the database
func (d *DbMongo) getLatestAttestedCommitmentHash() (chainhash.Hash, error) {

    sortFilter := bson.NewDocument(bson.EC.Int32("inserted_at", -1))
    attestationDoc := bson.NewDocument()
    resErr := d.db.Collection(COL_NAME_ATTESTATION).FindOne(d.ctx, bson.NewDocument(), findopt.Sort(sortFilter)).Decode(attestationDoc)
    if resErr != nil {
        fmt.Printf("couldn't get latest: %v\n", resErr)
        return chainhash.Hash{}, resErr
    }

    merkle_root := attestationDoc.Lookup("merkle_root").StringValue()
    commitmentHash, errHash := chainhash.NewHashFromStr(merkle_root)
    if errHash != nil {
        fmt.Printf("bad data in merkle_root column: %s\n", merkle_root)
        return chainhash.Hash{}, errHash
    }
	return *commitmentHash, nil
}

// Return latest from the database
func (d *DbMongo) getLatestCommitment() (models.Commitment, error) {

    sortFilter := bson.NewDocument(bson.EC.Int32("client_position", 1))
    res, resErr := d.db.Collection(COL_NAME_LATEST_COMMITMENT).Find(d.ctx, bson.NewDocument(), findopt.Sort(sortFilter))
    if resErr != nil {
        fmt.Printf("couldn't get latest: %v\n", resErr)
        return models.Commitment{}, resErr
    }

    var commitmentHashes []chainhash.Hash

    for res.Next(d.ctx) {
        commitmentDoc := bson.NewDocument()
        if err := res.Decode(commitmentDoc); err != nil {
            fmt.Printf("bad data in %s table: %s\n", COL_NAME_LATEST_COMMITMENT, res)
            return models.Commitment{}, err
        }
        commitment := commitmentDoc.Lookup("commitment").StringValue()
        commitmentHash, errHash := chainhash.NewHashFromStr(commitment)
        if errHash != nil {
            fmt.Printf("bad data in commitment column: %s\n", commitment)
            return models.Commitment{}, errHash
        }
        commitmentHashes = append(commitmentHashes, *commitmentHash)
    }
    if err := res.Err(); err != nil {
        return models.Commitment{}, fmt.Errorf("could not decode data: %v", err)
    }

    commitment, errCommitment := models.NewCommitment(commitmentHashes)
    if errCommitment != nil {
        return models.Commitment{}, errCommitment
    }
    return *commitment, nil
}

// Return latest from the database
func (d *DbMongo) getAttestationCommitment(attestationHash chainhash.Hash) (models.Commitment, error) {
    filterAttestation := bson.NewDocument(bson.EC.String("txid", attestationHash.String()))
    attestationDoc := bson.NewDocument()
    resErr := d.db.Collection(COL_NAME_ATTESTATION).FindOne(d.ctx, filterAttestation).Decode(attestationDoc)
    if resErr != nil {
        fmt.Printf("couldn't get latest: %v\n", resErr)
        return models.Commitment{}, resErr
    }

    sortFilter := bson.NewDocument(bson.EC.Int32("client_position", 1))
    merkle_root := attestationDoc.Lookup("merkle_root").StringValue()
    filterMerkleRoot := bson.NewDocument(bson.EC.String("merkle_root", merkle_root))
    res, resErr := d.db.Collection(COL_NAME_LATEST_COMMITMENT).Find(d.ctx, filterMerkleRoot, findopt.Sort(sortFilter))
    if resErr != nil {
        fmt.Printf("couldn't get latest: %v\n", resErr)
        return models.Commitment{}, resErr
    }

    var commitmentHashes []chainhash.Hash
    for res.Next(d.ctx) {
        commitmentDoc := bson.NewDocument()
        if err := res.Decode(commitmentDoc); err != nil {
            fmt.Printf("bad data in %s table: %s\n", COL_NAME_LATEST_COMMITMENT, res)
            return models.Commitment{}, err
        }
        fmt.Printf("%v\n", commitmentDoc)
        commitment := commitmentDoc.Lookup("commitment").StringValue()
        commitmentHash, errHash := chainhash.NewHashFromStr(commitment)
        if errHash != nil {
            fmt.Printf("bad data in commitment column: %s\n", commitment)
            return models.Commitment{}, errHash
        }
        commitmentHashes = append(commitmentHashes, *commitmentHash)
    }
    if err := res.Err(); err != nil {
        return models.Commitment{}, fmt.Errorf("could not decode data: %v", err)
    }

    commitment, errCommitment := models.NewCommitment(commitmentHashes)
    if errCommitment != nil {
        return models.Commitment{}, errCommitment
    }
    return *commitment, nil
}
