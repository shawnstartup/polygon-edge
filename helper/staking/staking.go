package staking

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/helper/keccak"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/0xPolygon/polygon-edge/validators"
)

var (
	MinValidatorCount = uint64(1)
	MaxValidatorCount = common.MaxSafeJSInt
)

// getAddressMapping returns the key for the SC storage mapping (address => something)
//
// More information:
// https://docs.soliditylang.org/en/latest/internals/layout_in_storage.html
func getAddressMapping(address types.Address, slot int64) []byte {
	bigSlot := big.NewInt(slot)

	finalSlice := append(
		common.PadLeftOrTrim(address.Bytes(), 32),
		common.PadLeftOrTrim(bigSlot.Bytes(), 32)...,
	)

	return keccak.Keccak256(nil, finalSlice)
}

// getIndexWithOffset is a helper method for adding an offset to the already found keccak hash
func getIndexWithOffset(keccakHash []byte, offset uint64) []byte {
	bigOffset := big.NewInt(int64(offset))
	bigKeccak := big.NewInt(0).SetBytes(keccakHash)

	bigKeccak.Add(bigKeccak, bigOffset)

	return bigKeccak.Bytes()
}

// getStorageIndexes is a helper function for getting the correct indexes
// of the storage slots which need to be modified during bootstrap.
//
// It is SC dependant, and based on the SC located at:
// https://github.com/0xPolygon/staking-contracts/
func getStorageIndexes(validator validators.Validator, index int) *StorageIndexes {
	storageIndexes := &StorageIndexes{}
	address := validator.Addr()

	// Get the indexes for the mappings
	// The index for the mapping is retrieved with:
	// keccak(address . slot)
	// . stands for concatenation (basically appending the bytes)
	storageIndexes.AddressToIsValidatorIndex = getAddressMapping(
		address,
		addressToIsValidatorSlot,
	)

	storageIndexes.AddressToStakedAmountIndex = getAddressMapping(
		address,
		addressToStakedAmountSlot,
	)

	storageIndexes.AddressToValidatorIndexIndex = getAddressMapping(
		address,
		addressToValidatorIndexSlot,
	)

	storageIndexes.ValidatorBLSPublicKeyIndex = getAddressMapping(
		address,
		addressToBLSPublicKeySlot,
	)

	// Index for array types is calculated as keccak(slot) + index
	// The slot for the dynamic arrays that's put in the keccak needs to be in hex form (padded 64 chars)
	storageIndexes.ValidatorsIndex = getIndexWithOffset(
		keccak.Keccak256(nil, common.PadLeftOrTrim(big.NewInt(validatorsSlot).Bytes(), 32)),
		uint64(index),
	)

	return storageIndexes
}

// setBytesToStorage sets bytes data into storage map from specified base index
func setBytesToStorage(
	storageMap map[types.Hash]types.Hash,
	baseIndexBytes []byte,
	data []byte,
) {
	dataLen := len(data)
	baseIndex := types.BytesToHash(baseIndexBytes)

	if dataLen <= 31 {
		bytes := types.Hash{}

		copy(bytes[:len(data)], data)

		// Set 2*Size at the first byte
		bytes[len(bytes)-1] = byte(dataLen * 2)

		storageMap[baseIndex] = bytes

		return
	}

	// Set size at the base index
	baseSlot := types.Hash{}
	baseSlot[31] = byte(2*dataLen + 1)
	storageMap[baseIndex] = baseSlot

	zeroIndex := keccak.Keccak256(nil, baseIndexBytes)
	numBytesInSlot := 256 / 8

	for i := 0; i < dataLen; i++ {
		offset := i / numBytesInSlot

		slotIndex := types.BytesToHash(getIndexWithOffset(zeroIndex, uint64(offset)))
		byteIndex := i % numBytesInSlot

		slot := storageMap[slotIndex]
		slot[byteIndex] = data[i]

		storageMap[slotIndex] = slot
	}
}

// PredeployParams contains the values used to predeploy the PoS staking contract
type PredeployParams struct {
	MinValidatorCount uint64
	MaxValidatorCount uint64
}

// StorageIndexes is a wrapper for different storage indexes that
// need to be modified
type StorageIndexes struct {
	ValidatorsIndex              []byte // []address
	ValidatorBLSPublicKeyIndex   []byte // mapping(address => byte[])
	AddressToIsValidatorIndex    []byte // mapping(address => bool)
	AddressToStakedAmountIndex   []byte // mapping(address => uint256)
	AddressToValidatorIndexIndex []byte // mapping(address => uint256)
}

// Slot definitions for SC storage
var (
	validatorsSlot              = int64(0) // Slot 0
	addressToIsValidatorSlot    = int64(1) // Slot 1
	addressToStakedAmountSlot   = int64(2) // Slot 2
	addressToValidatorIndexSlot = int64(3) // Slot 3
	stakedAmountSlot            = int64(4) // Slot 4
	minNumValidatorSlot         = int64(5) // Slot 5
	maxNumValidatorSlot         = int64(6) // Slot 6
	addressToBLSPublicKeySlot   = int64(7) // Slot 7
)

const (
	DefaultStakedBalance = "0x8AC7230489E80000" // 10 ETH
	//nolint: lll
	//StakingSCBytecode = "0x6080604052600436106101185760003560e01c80637a6eea37116100a0578063d94c111b11610064578063d94c111b1461040a578063e387a7ed14610433578063e804fbf61461045e578063f90ecacc14610489578063facd743b146104c657610186565b80637a6eea37146103215780637dceceb81461034c578063af6da36e14610389578063c795c077146103b4578063ca1e7819146103df57610186565b8063373d6132116100e7578063373d6132146102595780633a4b66f1146102845780633c561f041461028e57806351a9ab32146102b9578063714ff425146102f657610186565b806302b751991461018b578063065ae171146101c85780632367f6b5146102055780632def66201461024257610186565b366101865761013c3373ffffffffffffffffffffffffffffffffffffffff16610503565b1561017c576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016101739061178a565b60405180910390fd5b610184610516565b005b600080fd5b34801561019757600080fd5b506101b260048036038101906101ad9190611380565b6105ed565b6040516101bf91906117e5565b60405180910390f35b3480156101d457600080fd5b506101ef60048036038101906101ea9190611380565b610605565b6040516101fc91906116ed565b60405180910390f35b34801561021157600080fd5b5061022c60048036038101906102279190611380565b610625565b60405161023991906117e5565b60405180910390f35b34801561024e57600080fd5b5061025761066e565b005b34801561026557600080fd5b5061026e610759565b60405161027b91906117e5565b60405180910390f35b61028c610763565b005b34801561029a57600080fd5b506102a36107cc565b6040516102b091906116cb565b60405180910390f35b3480156102c557600080fd5b506102e060048036038101906102db9190611380565b610972565b6040516102ed9190611708565b60405180910390f35b34801561030257600080fd5b5061030b610a12565b60405161031891906117e5565b60405180910390f35b34801561032d57600080fd5b50610336610a1c565b60405161034391906117ca565b60405180910390f35b34801561035857600080fd5b50610373600480360381019061036e9190611380565b610a28565b60405161038091906117e5565b60405180910390f35b34801561039557600080fd5b5061039e610a40565b6040516103ab91906117e5565b60405180910390f35b3480156103c057600080fd5b506103c9610a46565b6040516103d691906117e5565b60405180910390f35b3480156103eb57600080fd5b506103f4610a4c565b60405161040191906116a9565b60405180910390f35b34801561041657600080fd5b50610431600480360381019061042c91906113ad565b610ada565b005b34801561043f57600080fd5b50610448610b31565b60405161045591906117e5565b60405180910390f35b34801561046a57600080fd5b50610473610b37565b60405161048091906117e5565b60405180910390f35b34801561049557600080fd5b506104b060048036038101906104ab91906113f6565b610b41565b6040516104bd919061168e565b60405180910390f35b3480156104d257600080fd5b506104ed60048036038101906104e89190611380565b610b80565b6040516104fa91906116ed565b60405180910390f35b600080823b905060008111915050919050565b34600460008282546105289190611906565b9250508190555034600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461057e9190611906565b9250508190555061058e33610bd6565b1561059d5761059c33610c4e565b5b3373ffffffffffffffffffffffffffffffffffffffff167f9e71bc8eea02a63969f509818f2dafb9254532904319f9dbda79b67bd34a5f3d346040516105e391906117e5565b60405180910390a2565b60036020528060005260406000206000915090505481565b60016020528060005260406000206000915054906101000a900460ff1681565b6000600260008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b61068d3373ffffffffffffffffffffffffffffffffffffffff16610503565b156106cd576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016106c49061178a565b60405180910390fd5b6000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020541161074f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107469061172a565b60405180910390fd5b610757610d9d565b565b6000600454905090565b6107823373ffffffffffffffffffffffffffffffffffffffff16610503565b156107c2576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107b99061178a565b60405180910390fd5b6107ca610516565b565b60606000808054905067ffffffffffffffff8111156107ee576107ed611b9e565b5b60405190808252806020026020018201604052801561082157816020015b606081526020019060019003908161080c5790505b50905060005b60008054905081101561096a576007600080838154811061084b5761084a611b6f565b5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002080546108bb90611a36565b80601f01602080910402602001604051908101604052809291908181526020018280546108e790611a36565b80156109345780601f1061090957610100808354040283529160200191610934565b820191906000526020600020905b81548152906001019060200180831161091757829003601f168201915b505050505082828151811061094c5761094b611b6f565b5b6020026020010181905250808061096290611a99565b915050610827565b508091505090565b6007602052806000526040600020600091509050805461099190611a36565b80601f01602080910402602001604051908101604052809291908181526020018280546109bd90611a36565b8015610a0a5780601f106109df57610100808354040283529160200191610a0a565b820191906000526020600020905b8154815290600101906020018083116109ed57829003601f168201915b505050505081565b6000600554905090565b670de0b6b3a764000081565b60026020528060005260406000206000915090505481565b60065481565b60055481565b60606000805480602002602001604051908101604052809291908181526020018280548015610ad057602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610a86575b5050505050905090565b80600760003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000209080519060200190610b2d929190611243565b5050565b60045481565b6000600654905090565b60008181548110610b5157600080fd5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b6000610be182610eef565b158015610c475750670de0b6b3a76400006fffffffffffffffffffffffffffffffff16600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410155b9050919050565b60065460008054905010610c97576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610c8e9061174a565b60405180910390fd5b60018060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff021916908315150217905550600080549050600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055506000819080600181540180825580915050600190039060005260206000200160009091909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b6000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508060046000828254610e38919061195c565b92505081905550610e4833610eef565b15610e5757610e5633610f45565b5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050158015610e9d573d6000803e3d6000fd5b503373ffffffffffffffffffffffffffffffffffffffff167f0f5bb82176feb1b5e747e28471aa92156a04d9f3ab9f45f28e2d704232b93f7582604051610ee491906117e5565b60405180910390a250565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b60055460008054905011610f8e576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610f85906117aa565b60405180910390fd5b600080549050600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410611014576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161100b9061176a565b60405180910390fd5b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600160008054905061106c919061195c565b905080821461115a57600080828154811061108a57611089611b6f565b5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905080600084815481106110cc576110cb611b6f565b5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555082600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550505b6000600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff0219169083151502179055506000600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600080548061120957611208611b40565b5b6001900381819060005260206000200160006101000a81549073ffffffffffffffffffffffffffffffffffffffff02191690559055505050565b82805461124f90611a36565b90600052602060002090601f01602090048101928261127157600085556112b8565b82601f1061128a57805160ff19168380011785556112b8565b828001600101855582156112b8579182015b828111156112b757825182559160200191906001019061129c565b5b5090506112c591906112c9565b5090565b5b808211156112e25760008160009055506001016112ca565b5090565b60006112f96112f484611825565b611800565b90508281526020810184848401111561131557611314611bd2565b5b6113208482856119f4565b509392505050565b60008135905061133781611d0b565b92915050565b600082601f83011261135257611351611bcd565b5b81356113628482602086016112e6565b91505092915050565b60008135905061137a81611d22565b92915050565b60006020828403121561139657611395611bdc565b5b60006113a484828501611328565b91505092915050565b6000602082840312156113c3576113c2611bdc565b5b600082013567ffffffffffffffff8111156113e1576113e0611bd7565b5b6113ed8482850161133d565b91505092915050565b60006020828403121561140c5761140b611bdc565b5b600061141a8482850161136b565b91505092915050565b600061142f838361144f565b60208301905092915050565b6000611447838361154f565b905092915050565b61145881611990565b82525050565b61146781611990565b82525050565b600061147882611876565b61148281856118b1565b935061148d83611856565b8060005b838110156114be5781516114a58882611423565b97506114b083611897565b925050600181019050611491565b5085935050505092915050565b60006114d682611881565b6114e081856118c2565b9350836020820285016114f285611866565b8060005b8581101561152e578484038952815161150f858261143b565b945061151a836118a4565b925060208a019950506001810190506114f6565b50829750879550505050505092915050565b611549816119a2565b82525050565b600061155a8261188c565b61156481856118d3565b9350611574818560208601611a03565b61157d81611be1565b840191505092915050565b60006115938261188c565b61159d81856118e4565b93506115ad818560208601611a03565b6115b681611be1565b840191505092915050565b60006115ce601d836118f5565b91506115d982611bf2565b602082019050919050565b60006115f16027836118f5565b91506115fc82611c1b565b604082019050919050565b60006116146012836118f5565b915061161f82611c6a565b602082019050919050565b6000611637601a836118f5565b915061164282611c93565b602082019050919050565b600061165a6040836118f5565b915061166582611cbc565b604082019050919050565b611679816119ae565b82525050565b611688816119ea565b82525050565b60006020820190506116a3600083018461145e565b92915050565b600060208201905081810360008301526116c3818461146d565b905092915050565b600060208201905081810360008301526116e581846114cb565b905092915050565b60006020820190506117026000830184611540565b92915050565b600060208201905081810360008301526117228184611588565b905092915050565b60006020820190508181036000830152611743816115c1565b9050919050565b60006020820190508181036000830152611763816115e4565b9050919050565b6000602082019050818103600083015261178381611607565b9050919050565b600060208201905081810360008301526117a38161162a565b9050919050565b600060208201905081810360008301526117c38161164d565b9050919050565b60006020820190506117df6000830184611670565b92915050565b60006020820190506117fa600083018461167f565b92915050565b600061180a61181b565b90506118168282611a68565b919050565b6000604051905090565b600067ffffffffffffffff8211156118405761183f611b9e565b5b61184982611be1565b9050602081019050919050565b6000819050602082019050919050565b6000819050602082019050919050565b600081519050919050565b600081519050919050565b600081519050919050565b6000602082019050919050565b6000602082019050919050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b6000611911826119ea565b915061191c836119ea565b9250827fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0382111561195157611950611ae2565b5b828201905092915050565b6000611967826119ea565b9150611972836119ea565b92508282101561198557611984611ae2565b5b828203905092915050565b600061199b826119ca565b9050919050565b60008115159050919050565b60006fffffffffffffffffffffffffffffffff82169050919050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000819050919050565b82818337600083830152505050565b60005b83811015611a21578082015181840152602081019050611a06565b83811115611a30576000848401525b50505050565b60006002820490506001821680611a4e57607f821691505b60208210811415611a6257611a61611b11565b5b50919050565b611a7182611be1565b810181811067ffffffffffffffff82111715611a9057611a8f611b9e565b5b80604052505050565b6000611aa4826119ea565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff821415611ad757611ad6611ae2565b5b600182019050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600080fd5b600080fd5b600080fd5b600080fd5b6000601f19601f8301169050919050565b7f4f6e6c79207374616b65722063616e2063616c6c2066756e6374696f6e000000600082015250565b7f56616c696461746f72207365742068617320726561636865642066756c6c206360008201527f6170616369747900000000000000000000000000000000000000000000000000602082015250565b7f696e646578206f7574206f662072616e67650000000000000000000000000000600082015250565b7f4f6e6c7920454f412063616e2063616c6c2066756e6374696f6e000000000000600082015250565b7f56616c696461746f72732063616e2774206265206c657373207468616e20746860008201527f65206d696e696d756d2072657175697265642076616c696461746f72206e756d602082015250565b611d1481611990565b8114611d1f57600080fd5b50565b611d2b816119ea565b8114611d3657600080fd5b5056fea26469706673582212201556e5927c99f1e21e8ae2bbc55b0b507bc60d9732fc9a5e25a0708b409c8c8064736f6c63430008070033"

	//DefaultStakedBalance = "0x152d02c7e14af6800000" // 100,000 ETH
	//nolint: lll
	StakingSCBytecode = "0x6080604052600436106101185760003560e01c80637a6eea37116100a0578063d94c111b11610064578063d94c111b1461040a578063e387a7ed14610433578063e804fbf61461045e578063f90ecacc14610489578063facd743b146104c657610186565b80637a6eea37146103215780637dceceb81461034c578063af6da36e14610389578063c795c077146103b4578063ca1e7819146103df57610186565b8063373d6132116100e7578063373d6132146102595780633a4b66f1146102845780633c561f041461028e57806351a9ab32146102b9578063714ff425146102f657610186565b806302b751991461018b578063065ae171146101c85780632367f6b5146102055780632def66201461024257610186565b366101865761013c3373ffffffffffffffffffffffffffffffffffffffff16610503565b1561017c576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016101739061178a565b60405180910390fd5b610184610516565b005b600080fd5b34801561019757600080fd5b506101b260048036038101906101ad9190611380565b6105ed565b6040516101bf91906117e5565b60405180910390f35b3480156101d457600080fd5b506101ef60048036038101906101ea9190611380565b610605565b6040516101fc91906116ed565b60405180910390f35b34801561021157600080fd5b5061022c60048036038101906102279190611380565b610625565b60405161023991906117e5565b60405180910390f35b34801561024e57600080fd5b5061025761066e565b005b34801561026557600080fd5b5061026e610759565b60405161027b91906117e5565b60405180910390f35b61028c610763565b005b34801561029a57600080fd5b506102a36107cc565b6040516102b091906116cb565b60405180910390f35b3480156102c557600080fd5b506102e060048036038101906102db9190611380565b610972565b6040516102ed9190611708565b60405180910390f35b34801561030257600080fd5b5061030b610a12565b60405161031891906117e5565b60405180910390f35b34801561032d57600080fd5b50610336610a1c565b60405161034391906117ca565b60405180910390f35b34801561035857600080fd5b50610373600480360381019061036e9190611380565b610a28565b60405161038091906117e5565b60405180910390f35b34801561039557600080fd5b5061039e610a40565b6040516103ab91906117e5565b60405180910390f35b3480156103c057600080fd5b506103c9610a46565b6040516103d691906117e5565b60405180910390f35b3480156103eb57600080fd5b506103f4610a4c565b60405161040191906116a9565b60405180910390f35b34801561041657600080fd5b50610431600480360381019061042c91906113ad565b610ada565b005b34801561043f57600080fd5b50610448610b31565b60405161045591906117e5565b60405180910390f35b34801561046a57600080fd5b50610473610b37565b60405161048091906117e5565b60405180910390f35b34801561049557600080fd5b506104b060048036038101906104ab91906113f6565b610b41565b6040516104bd919061168e565b60405180910390f35b3480156104d257600080fd5b506104ed60048036038101906104e89190611380565b610b80565b6040516104fa91906116ed565b60405180910390f35b600080823b905060008111915050919050565b34600460008282546105289190611906565b9250508190555034600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461057e9190611906565b9250508190555061058e33610bd6565b1561059d5761059c33610c4e565b5b3373ffffffffffffffffffffffffffffffffffffffff167f9e71bc8eea02a63969f509818f2dafb9254532904319f9dbda79b67bd34a5f3d346040516105e391906117e5565b60405180910390a2565b60036020528060005260406000206000915090505481565b60016020528060005260406000206000915054906101000a900460ff1681565b6000600260008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b61068d3373ffffffffffffffffffffffffffffffffffffffff16610503565b156106cd576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016106c49061178a565b60405180910390fd5b6000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020541161074f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107469061172a565b60405180910390fd5b610757610d9d565b565b6000600454905090565b6107823373ffffffffffffffffffffffffffffffffffffffff16610503565b156107c2576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016107b99061178a565b60405180910390fd5b6107ca610516565b565b60606000808054905067ffffffffffffffff8111156107ee576107ed611b9e565b5b60405190808252806020026020018201604052801561082157816020015b606081526020019060019003908161080c5790505b50905060005b60008054905081101561096a576007600080838154811061084b5761084a611b6f565b5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002080546108bb90611a36565b80601f01602080910402602001604051908101604052809291908181526020018280546108e790611a36565b80156109345780601f1061090957610100808354040283529160200191610934565b820191906000526020600020905b81548152906001019060200180831161091757829003601f168201915b505050505082828151811061094c5761094b611b6f565b5b6020026020010181905250808061096290611a99565b915050610827565b508091505090565b6007602052806000526040600020600091509050805461099190611a36565b80601f01602080910402602001604051908101604052809291908181526020018280546109bd90611a36565b8015610a0a5780601f106109df57610100808354040283529160200191610a0a565b820191906000526020600020905b8154815290600101906020018083116109ed57829003601f168201915b505050505081565b6000600554905090565b670de0b6b3a764000081565b60026020528060005260406000206000915090505481565b60065481565b60055481565b60606000805480602002602001604051908101604052809291908181526020018280548015610ad057602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610a86575b5050505050905090565b80600760003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000209080519060200190610b2d929190611243565b5050565b60045481565b6000600654905090565b60008181548110610b5157600080fd5b906000526020600020016000915054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b6000610be182610eef565b158015610c475750670de0b6b3a76400006fffffffffffffffffffffffffffffffff16600260008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410155b9050919050565b60065460008054905010610c97576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610c8e9061174a565b60405180910390fd5b60018060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff021916908315150217905550600080549050600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055506000819080600181540180825580915050600190039060005260206000200160009091909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b6000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600260003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508060046000828254610e38919061195c565b92505081905550610e4833610eef565b15610e5757610e5633610f45565b5b3373ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050158015610e9d573d6000803e3d6000fd5b503373ffffffffffffffffffffffffffffffffffffffff167f0f5bb82176feb1b5e747e28471aa92156a04d9f3ab9f45f28e2d704232b93f7582604051610ee491906117e5565b60405180910390a250565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff169050919050565b60055460008054905011610f8e576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401610f85906117aa565b60405180910390fd5b600080549050600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410611014576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161100b9061176a565b60405180910390fd5b6000600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600160008054905061106c919061195c565b905080821461115a57600080828154811061108a57611089611b6f565b5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16905080600084815481106110cc576110cb611b6f565b5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555082600360008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550505b6000600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff0219169083151502179055506000600360008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600080548061120957611208611b40565b5b6001900381819060005260206000200160006101000a81549073ffffffffffffffffffffffffffffffffffffffff02191690559055505050565b82805461124f90611a36565b90600052602060002090601f01602090048101928261127157600085556112b8565b82601f1061128a57805160ff19168380011785556112b8565b828001600101855582156112b8579182015b828111156112b757825182559160200191906001019061129c565b5b5090506112c591906112c9565b5090565b5b808211156112e25760008160009055506001016112ca565b5090565b60006112f96112f484611825565b611800565b90508281526020810184848401111561131557611314611bd2565b5b6113208482856119f4565b509392505050565b60008135905061133781611d0b565b92915050565b600082601f83011261135257611351611bcd565b5b81356113628482602086016112e6565b91505092915050565b60008135905061137a81611d22565b92915050565b60006020828403121561139657611395611bdc565b5b60006113a484828501611328565b91505092915050565b6000602082840312156113c3576113c2611bdc565b5b600082013567ffffffffffffffff8111156113e1576113e0611bd7565b5b6113ed8482850161133d565b91505092915050565b60006020828403121561140c5761140b611bdc565b5b600061141a8482850161136b565b91505092915050565b600061142f838361144f565b60208301905092915050565b6000611447838361154f565b905092915050565b61145881611990565b82525050565b61146781611990565b82525050565b600061147882611876565b61148281856118b1565b935061148d83611856565b8060005b838110156114be5781516114a58882611423565b97506114b083611897565b925050600181019050611491565b5085935050505092915050565b60006114d682611881565b6114e081856118c2565b9350836020820285016114f285611866565b8060005b8581101561152e578484038952815161150f858261143b565b945061151a836118a4565b925060208a019950506001810190506114f6565b50829750879550505050505092915050565b611549816119a2565b82525050565b600061155a8261188c565b61156481856118d3565b9350611574818560208601611a03565b61157d81611be1565b840191505092915050565b60006115938261188c565b61159d81856118e4565b93506115ad818560208601611a03565b6115b681611be1565b840191505092915050565b60006115ce601d836118f5565b91506115d982611bf2565b602082019050919050565b60006115f16027836118f5565b91506115fc82611c1b565b604082019050919050565b60006116146012836118f5565b915061161f82611c6a565b602082019050919050565b6000611637601a836118f5565b915061164282611c93565b602082019050919050565b600061165a6040836118f5565b915061166582611cbc565b604082019050919050565b611679816119ae565b82525050565b611688816119ea565b82525050565b60006020820190506116a3600083018461145e565b92915050565b600060208201905081810360008301526116c3818461146d565b905092915050565b600060208201905081810360008301526116e581846114cb565b905092915050565b60006020820190506117026000830184611540565b92915050565b600060208201905081810360008301526117228184611588565b905092915050565b60006020820190508181036000830152611743816115c1565b9050919050565b60006020820190508181036000830152611763816115e4565b9050919050565b6000602082019050818103600083015261178381611607565b9050919050565b600060208201905081810360008301526117a38161162a565b9050919050565b600060208201905081810360008301526117c38161164d565b9050919050565b60006020820190506117df6000830184611670565b92915050565b60006020820190506117fa600083018461167f565b92915050565b600061180a61181b565b90506118168282611a68565b919050565b6000604051905090565b600067ffffffffffffffff8211156118405761183f611b9e565b5b61184982611be1565b9050602081019050919050565b6000819050602082019050919050565b6000819050602082019050919050565b600081519050919050565b600081519050919050565b600081519050919050565b6000602082019050919050565b6000602082019050919050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b6000611911826119ea565b915061191c836119ea565b9250827fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0382111561195157611950611ae2565b5b828201905092915050565b6000611967826119ea565b9150611972836119ea565b92508282101561198557611984611ae2565b5b828203905092915050565b600061199b826119ca565b9050919050565b60008115159050919050565b60006fffffffffffffffffffffffffffffffff82169050919050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000819050919050565b82818337600083830152505050565b60005b83811015611a21578082015181840152602081019050611a06565b83811115611a30576000848401525b50505050565b60006002820490506001821680611a4e57607f821691505b60208210811415611a6257611a61611b11565b5b50919050565b611a7182611be1565b810181811067ffffffffffffffff82111715611a9057611a8f611b9e565b5b80604052505050565b6000611aa4826119ea565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff821415611ad757611ad6611ae2565b5b600182019050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600080fd5b600080fd5b600080fd5b600080fd5b6000601f19601f8301169050919050565b7f4f6e6c79207374616b65722063616e2063616c6c2066756e6374696f6e000000600082015250565b7f56616c696461746f72207365742068617320726561636865642066756c6c206360008201527f6170616369747900000000000000000000000000000000000000000000000000602082015250565b7f696e646578206f7574206f662072616e67650000000000000000000000000000600082015250565b7f4f6e6c7920454f412063616e2063616c6c2066756e6374696f6e000000000000600082015250565b7f56616c696461746f72732063616e2774206265206c657373207468616e20746860008201527f65206d696e696d756d2072657175697265642076616c696461746f72206e756d602082015250565b611d1481611990565b8114611d1f57600080fd5b50565b611d2b816119ea565b8114611d3657600080fd5b5056fea26469706673582212201556e5927c99f1e21e8ae2bbc55b0b507bc60d9732fc9a5e25a0708b409c8c8064736f6c63430008070033"
)

// PredeployStakingSC is a helper method for setting up the staking smart contract account,
// using the passed in validators as pre-staked validators
func PredeployStakingSC(
	vals validators.Validators,
	params PredeployParams,
) (*chain.GenesisAccount, error) {
	// Set the code for the staking smart contract
	// Code retrieved from https://github.com/0xPolygon/staking-contracts
	scHex, _ := hex.DecodeHex(StakingSCBytecode)
	stakingAccount := &chain.GenesisAccount{
		Code: scHex,
	}

	// Parse the default staked balance value into *big.Int
	val := DefaultStakedBalance
	bigDefaultStakedBalance, err := types.ParseUint256orHex(&val)

	if err != nil {
		return nil, fmt.Errorf("unable to generate DefaultStatkedBalance, %w", err)
	}

	// Generate the empty account storage map
	storageMap := make(map[types.Hash]types.Hash)
	bigTrueValue := big.NewInt(1)
	stakedAmount := big.NewInt(0)
	bigMinNumValidators := big.NewInt(int64(params.MinValidatorCount))
	bigMaxNumValidators := big.NewInt(int64(params.MaxValidatorCount))
	valsLen := big.NewInt(0)

	if vals != nil {
		valsLen = big.NewInt(int64(vals.Len()))

		for idx := 0; idx < vals.Len(); idx++ {
			validator := vals.At(uint64(idx))

			// Update the total staked amount
			stakedAmount = stakedAmount.Add(stakedAmount, bigDefaultStakedBalance)

			// Get the storage indexes
			storageIndexes := getStorageIndexes(validator, idx)

			// Set the value for the validators array
			storageMap[types.BytesToHash(storageIndexes.ValidatorsIndex)] =
				types.BytesToHash(
					validator.Addr().Bytes(),
				)

			if blsValidator, ok := validator.(*validators.BLSValidator); ok {
				setBytesToStorage(
					storageMap,
					storageIndexes.ValidatorBLSPublicKeyIndex,
					blsValidator.BLSPublicKey,
				)
			}

			// Set the value for the address -> validator array index mapping
			storageMap[types.BytesToHash(storageIndexes.AddressToIsValidatorIndex)] =
				types.BytesToHash(bigTrueValue.Bytes())

			// Set the value for the address -> staked amount mapping
			storageMap[types.BytesToHash(storageIndexes.AddressToStakedAmountIndex)] =
				types.StringToHash(hex.EncodeBig(bigDefaultStakedBalance))

			// Set the value for the address -> validator index mapping
			storageMap[types.BytesToHash(storageIndexes.AddressToValidatorIndexIndex)] =
				types.StringToHash(hex.EncodeUint64(uint64(idx)))
		}
	}

	// Set the value for the total staked amount
	storageMap[types.BytesToHash(big.NewInt(stakedAmountSlot).Bytes())] =
		types.BytesToHash(stakedAmount.Bytes())

	// Set the value for the size of the validators array
	storageMap[types.BytesToHash(big.NewInt(validatorsSlot).Bytes())] =
		types.BytesToHash(valsLen.Bytes())

	// Set the value for the minimum number of validators
	storageMap[types.BytesToHash(big.NewInt(minNumValidatorSlot).Bytes())] =
		types.BytesToHash(bigMinNumValidators.Bytes())

	// Set the value for the maximum number of validators
	storageMap[types.BytesToHash(big.NewInt(maxNumValidatorSlot).Bytes())] =
		types.BytesToHash(bigMaxNumValidators.Bytes())

	// Save the storage map
	stakingAccount.Storage = storageMap

	// Set the Staking SC balance to numValidators * defaultStakedBalance
	stakingAccount.Balance = stakedAmount

	return stakingAccount, nil
}
