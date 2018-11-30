// Copyright (c) 2018 CommerceBlock Team
// Use of this source code is governed by an MIT
// license that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/assert"
)

// Test various Config error cases
func TestConfigErrors(t *testing.T) {
	var configErr error
	var testConf = []byte(`
    {
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_NAME_NOT_FOUND, MAIN_CHAIN_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, RPC_CLIENT_URL_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, RPC_CLIENT_USER_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, RPC_CLIENT_PASS_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, RPC_CLIENT_CHAIN_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "testnet"
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "allaloum"
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "testnet"
        },
        "db": {
            "user": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, DB_PASSWORD_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "testnet"
        },
        "db": {
            "user": "",
            "password": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, DB_HOST_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "testnet"
        },
        "db": {
            "user": "",
            "password": "",
            "host": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, DB_PORT_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "testnet"
        },
        "db": {
            "user": "",
            "password": "",
            "host": "",
            "port": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, DB_NAME_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": "testnet"
        },
        "db": {
            "user": "",
            "password": "",
            "host": "",
            "port": "",
            "name": ""
        }
    }
    `)
	_, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
}

// Test actual Config parses correct values
func TestConfigActual(t *testing.T) {
	var configErr error
	var config *Config
	var testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "signer": {
            "signers": "127.0.0.1:12345,127.0.0.1:12346"
        },
        "db": {
            "user":"username1",
            "password":"password2",
            "host":"localhost",
            "port":"27017",
            "name":"mainstay"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)

	assert.Equal(t, true, config.MainClient() != nil)
	assert.Equal(t, &chaincfg.RegressionNetParams, config.MainChainCfg())
	assert.Equal(t, []string{"127.0.0.1:12345", "127.0.0.1:12346"}, config.SignerConfig().Signers)
	assert.Equal(t, DbConfig{
		User:     "username1",
		Password: "password2",
		Host:     "localhost",
		Port:     "27017",
		Name:     "mainstay",
	}, config.DbConfig())
}

// Test config for Optional staychain parameters
func TestConfigStaychain(t *testing.T) {
	var configErr error
	var config *Config
	var testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": ""
        },
        "staychain": {
            "initTx": "87e56bda501ba6a022f12e178e9f1ac03fb2c07f04e1dfa62ac9e1d83cd840e1",
            "initScript": "51210381324c14a482646e9ad7cf82372021e5ecb9a7e1b67ee168dddf1e97dafe40af210376c091faaeb6bb3b74e0568db5dd499746d99437758a5cb1e60ab38f02e279c352ae",
            "topupTx": "97e56bda501ba6a022f12e178e9f1ac03fb2c07f04e1dfa62ac9e1d83cd840e1",
            "topupScript": "51210381324c14a482646e9ad7cf92372021e5ecb9a7e1b67ee168dddf1e97dafe40af210376c091faaeb6bb3b74e0568db5dd499746d99437758a5cb1e60ab38f02e279c352ae",
            "regtest": "1"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)

	assert.Equal(t, "87e56bda501ba6a022f12e178e9f1ac03fb2c07f04e1dfa62ac9e1d83cd840e1", config.InitTx())
	assert.Equal(t, "51210381324c14a482646e9ad7cf82372021e5ecb9a7e1b67ee168dddf1e97dafe40af210376c091faaeb6bb3b74e0568db5dd499746d99437758a5cb1e60ab38f02e279c352ae", config.InitScript())
	assert.Equal(t, "97e56bda501ba6a022f12e178e9f1ac03fb2c07f04e1dfa62ac9e1d83cd840e1", config.TopupTx())
	assert.Equal(t, "51210381324c14a482646e9ad7cf92372021e5ecb9a7e1b67ee168dddf1e97dafe40af210376c091faaeb6bb3b74e0568db5dd499746d99437758a5cb1e60ab38f02e279c352ae", config.TopupScript())
	assert.Equal(t, true, config.Regtest())

	config.SetRegtest(false)
	assert.Equal(t, false, config.Regtest())

	config.SetInitTx("aa")
	assert.Equal(t, "aa", config.InitTx())

	config.SetInitScript("bb")
	assert.Equal(t, "bb", config.InitScript())

	config.SetTopupTx("cc")
	assert.Equal(t, "cc", config.TopupTx())

	config.SetTopupScript("dd")
	assert.Equal(t, "dd", config.TopupScript())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": ""
        },
        "staychain": {
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)

	assert.Equal(t, "", config.InitTx())
	assert.Equal(t, "", config.InitScript())
	assert.Equal(t, "", config.TopupTx())
	assert.Equal(t, "", config.TopupScript())
	assert.Equal(t, false, config.Regtest())
}

// Test config for Optional fees parameters
func TestConfigFees(t *testing.T) {
	var configErr error
	var config *Config
	var testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "fees": {
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, FeesConfig{-1, -1, -1}, config.FeesConfig())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "fees": {
            "minFee": "1"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, FeesConfig{1, -1, -1}, config.FeesConfig())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "fees": {
            "minFee": "invalid"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, FeesConfig{-1, -1, -1}, config.FeesConfig())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "fees": {
            "maxFee": "10",
            "minFee": "5",
            "feeIncrement": "11",
            "something-else": "nice-value"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, FeesConfig{5, 10, 11}, config.FeesConfig())
}

// Test config for Optional timing parameters
func TestConfigTiming(t *testing.T) {
	var configErr error
	var config *Config
	var testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "timing": {
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, TimingConfig{-1, -1}, config.TimingConfig())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "timing": {
            "newAttestationMinutes": "0"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, TimingConfig{0, -1}, config.TimingConfig())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "timing": {
            "handleUnconfirmedMinutes": "0"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, TimingConfig{-1, 0}, config.TimingConfig())

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "localhost:18443",
            "rpcuser": "user",
            "rpcpass": "pass",
            "chain": "regtest"
        },
        "timing": {
            "newAttestationMinutes": "10",
            "handleUnconfirmedMinutes": "60"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, TimingConfig{10, 60}, config.TimingConfig())
}

// Test config for Optional signer parameters
func TestConfigSigner(t *testing.T) {
	var config *Config
	var configErr error
	var testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": ""
        },
        "signer": {
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, errors.New(fmt.Sprintf("%s: %s", ERROR_CONFIG_VALUE_NOT_FOUND, SIGNER_SIGNERS_NAME)), configErr)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": ""
        },
        "signer": {
            "signers": "host"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, []string{"host"}, config.SignerConfig().Signers)

	testConf = []byte(`
    {
        "main": {
            "rpcurl": "",
            "rpcuser": "",
            "rpcpass": "",
            "chain": ""
        },
        "signer": {
            "signers": "host",
            "publisher": "*:5000"
        }
    }
    `)
	config, configErr = NewConfig(testConf)
	assert.Equal(t, nil, configErr)
	assert.Equal(t, "*:5000", config.SignerConfig().Publisher)
}