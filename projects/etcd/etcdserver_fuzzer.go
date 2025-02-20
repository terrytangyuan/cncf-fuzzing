// Copyright 2021 ADA Logics Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package etcdserver

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"go.uber.org/zap/zaptest"

	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	membershippb "go.etcd.io/etcd/api/v3/membershippb"
	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3alarm"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	"go.etcd.io/etcd/server/v3/lease"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

var (
	ab                    applierV3
	tokenTypeSimple       = "simple"
	simpleTokenTTLDefault = 300 * time.Second
	ops                   = map[int]string{
		0:  "Range",
		1:  "Put",
		2:  "DeleteRange",
		3:  "Txn",
		4:  "Compaction",
		5:  "LeaseGrant",
		6:  "LeaseRevoke",
		7:  "Alarm",
		8:  "LeaseCheckpoint",
		9:  "AuthEnable",
		10: "AuthDisable",
		11: "AuthStatus",
		12: "Authenticate",
		13: "AuthUserAdd",
		14: "AuthUserDelete",
		15: "AuthUserGet",
		16: "AuthUserChangePassword",
		17: "AuthUserGrantRole",
		18: "AuthUserRevokeRole",
		19: "AuthUserList",
		20: "AuthRoleList",
		21: "AuthRoleAdd",
		22: "AuthRoleDelete",
		23: "AuthRoleGet",
		24: "AuthRoleGrantPermission",
		25: "AuthRoleRevokePermission",
		26: "ClusterVersionSet",
		27: "ClusterMemberAttrSet",
		28: "DowngradeInfoSet",
	}
)

func dummyIndexWaiter(index uint64) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		ch <- struct{}{}
	}()
	return ch
}

func validateRangeRequest(r *pb.RangeRequest) error {
	if r.Key == nil || r.RangeEnd == nil || &r.Limit == nil || &r.Revision == nil || &r.SortOrder == nil || &r.SortTarget == nil || &r.Serializable == nil || &r.KeysOnly == nil || &r.CountOnly == nil || &r.MinModRevision == nil || &r.MaxModRevision == nil {
		return fmt.Errorf("Not valid rangerequest")
	}
	return nil
}

func validatePutRequest(r *pb.PutRequest) error {
	if r.Key == nil || r.Value == nil || &r.Lease == nil || &r.PrevKv == nil || &r.IgnoreValue == nil || &r.IgnoreLease == nil {
		return fmt.Errorf("Not valid putrequest")
	}
	return nil
}

func validateDeleteRangeRequest(r *pb.DeleteRangeRequest) error {
	if r.Key == nil || r.RangeEnd == nil || &r.PrevKv == nil {
		return fmt.Errorf("Not valid DeleteRangeRequest")
	}
	return nil
}

func validateTxnRequest(r *pb.TxnRequest) error {
	if r.Compare == nil || r.Success == nil || r.Failure == nil {
		return fmt.Errorf("Not valid TxnRequest")
	}
	return nil
}

func validateCompactionRequest(r *pb.CompactionRequest) error {
	if &r.Revision == nil || &r.Physical == nil {
		return fmt.Errorf("Not valid CompactionRequest ")
	}
	return nil
}

func validateLeaseGrantRequest(r *pb.LeaseGrantRequest) error {
	if &r.TTL == nil || &r.ID == nil {
		return fmt.Errorf("Not valid LeaseGrantRequest")
	}
	return nil
}

func validateLeaseRevokeRequest(r *pb.LeaseRevokeRequest) error {
	if &r.ID == nil {
		return fmt.Errorf("Not valid LeaseRevokeRequest")
	}
	return fmt.Errorf("")
	return nil
}

func validateAlarmRequest(r *pb.AlarmRequest) error {
	if &r.Action == nil || &r.MemberID == nil || &r.Alarm == nil {
		return fmt.Errorf("Not valid AlarmRequest")
	}
	return nil
}

func validateLeaseCheckpointRequest(r *pb.LeaseCheckpointRequest) error {
	if r.Checkpoints == nil {
		return fmt.Errorf("Not valid LeaseCheckpointRequest")
	}
	return nil
}

func validateAuthEnableRequest(r *pb.AuthEnableRequest) error {
	return nil
}

func validateAuthDisableRequest(r *pb.AuthDisableRequest) error {
	return nil
}

func validateAuthStatusRequest(r *pb.AuthStatusRequest) error {
	return nil
}

func validateInternalAuthenticateRequest(r *pb.InternalAuthenticateRequest) error {
	if &r.Name == nil || &r.Password == nil || &r.SimpleToken == nil {
		return fmt.Errorf("Not a valid InternalAuthenticateRequest")
	}
	return nil
}

func validateAuthUserAddRequest(r *pb.AuthUserAddRequest) error {
	if &r.Name == nil || &r.Password == nil || &r.Options == nil || &r.HashedPassword == nil {
		return fmt.Errorf("Not a valid AuthUserAddRequest")
	}
	return nil
}

func validateAuthUserDeleteRequest(r *pb.AuthUserDeleteRequest) error {
	if &r.Name == nil {
		return fmt.Errorf("Not a valid AuthUserDeleteRequest")
	}
	return nil
}

func validateAuthUserGetRequest(r *pb.AuthUserGetRequest) error {
	if &r.Name == nil {
		return fmt.Errorf("Not a valid AuthUserGetRequest")
	}
	return nil
}

func validateAuthUserChangePasswordRequest(r *pb.AuthUserChangePasswordRequest) error {
	if &r.Name == nil || &r.Password == nil || &r.HashedPassword == nil {
		return fmt.Errorf("Not a valid AuthUserChangePasswordRequest")
	}
	return nil
}

func validateAuthUserGrantRoleRequest(r *pb.AuthUserGrantRoleRequest) error {
	if &r.User == nil || &r.Role == nil {
		return fmt.Errorf("Not a valid AuthUserGrantRoleRequest")
	}
	return nil
}

func validateAuthUserRevokeRoleRequest(r *pb.AuthUserRevokeRoleRequest) error {
	if &r.Name == nil || &r.Role == nil {
		return fmt.Errorf("Not a valid AuthUserRevokeRoleRequest")
	}
	return nil
}

func validateAuthUserListRequest(r *pb.AuthUserListRequest) error {
	return nil
}

func validateAuthRoleListRequest(r *pb.AuthRoleListRequest) error {
	return nil
}

func validateAuthRoleAddRequest(r *pb.AuthRoleAddRequest) error {
	if &r.Name == nil {
		return fmt.Errorf("Not a valid AuthRoleAddRequest")
	}
	return nil
}

func validateAuthRoleDeleteRequest(r *pb.AuthRoleDeleteRequest) error {
	if &r.Role == nil {
		return fmt.Errorf("Not a valid AuthRoleDeleteRequest")
	}
	return nil
}

func validateAuthRoleGetRequest(r *pb.AuthRoleGetRequest) error {
	if &r.Role == nil {
		return fmt.Errorf("Not a valid AuthRoleGetRequest")
	}
	return nil
}

func validateAuthRoleGrantPermissionRequest(r *pb.AuthRoleGrantPermissionRequest) error {
	if &r.Name == nil || &r.Perm == nil {
		return fmt.Errorf("Not a valid AuthRoleGrantPermissionRequest")
	}
	return nil
}

func validateAuthRoleRevokePermissionRequest(r *pb.AuthRoleRevokePermissionRequest) error {
	if &r.Role == nil || &r.Key == nil || &r.RangeEnd == nil {
		return fmt.Errorf("Not a valid AuthRoleRevokePermissionRequest")
	}
	return nil
}

func validateClusterVersionSetRequest(r *membershippb.ClusterVersionSetRequest) error {
	if &r.Ver == nil {
		return fmt.Errorf("Not a valid ClusterVersionSetRequest")
	}
	return nil
}

func validateClusterMemberAttrSetRequest(r *membershippb.ClusterMemberAttrSetRequest) error {
	if &r.Member_ID == nil || &r.MemberAttributes == nil {
		return fmt.Errorf("Not a valid ClusterMemberAttrSetRequest")
	}
	return nil
}

func validateDowngradeInfoSetRequest(r *membershippb.DowngradeInfoSetRequest) error {
	if &r.Enabled == nil || &r.Ver == nil {
		return fmt.Errorf("Not a valid DowngradeInfoSetRequest")
	}
	return nil
}

func setRequestType(internalRequest *pb.InternalRaftRequest, f *fuzz.ConsumeFuzzer) error {
	opInt, err := f.GetInt()
	if err != nil {
		return err
	}
	opType := ops[opInt%len(ops)]
	switch opType {
	case "Range":
		r := &pb.RangeRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateRangeRequest(r)
		if err != nil {
			return err
		}
		internalRequest.Range = r
	case "Put":
		r := &pb.PutRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validatePutRequest(r)
		if err != nil {
			return err
		}
		internalRequest.Put = r
	case "DeleteRange":
		r := &pb.DeleteRangeRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateDeleteRangeRequest(r)
		if err != nil {
			return err
		}
		internalRequest.DeleteRange = r
	case "Txn":
		r := &pb.TxnRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateTxnRequest(r)
		if err != nil {
			return err
		}
		internalRequest.Txn = r
	case "Compaction":
		r := &pb.CompactionRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateCompactionRequest(r)
		if err != nil {
			return err
		}
		internalRequest.Compaction = r
	case "LeaseGrant":
		r := &pb.LeaseGrantRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateLeaseGrantRequest(r)
		if err != nil {
			return err
		}
		internalRequest.LeaseGrant = r
	case "LeaseRevoke":
		r := &pb.LeaseRevokeRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateLeaseRevokeRequest(r)
		if err != nil {
			return err
		}
		internalRequest.LeaseRevoke = r
	case "Alarm":
		r := &pb.AlarmRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		internalRequest.Alarm = r
	case "LeaseCheckpoint":
		r := &pb.LeaseCheckpointRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateLeaseCheckpointRequest(r)
		if err != nil {
			return err
		}
		internalRequest.LeaseCheckpoint = r
	case "AuthEnable":
		r := &pb.AuthEnableRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthEnableRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthEnable = r
	case "AuthDisable":
		r := &pb.AuthDisableRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthDisableRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthDisable = r
	case "AuthStatus":
		r := &pb.AuthStatusRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthStatusRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthStatus = r
	case "Authenticate":
		r := &pb.InternalAuthenticateRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateInternalAuthenticateRequest(r)
		if err != nil {
			return err
		}
		internalRequest.Authenticate = r
	case "AuthUserAdd":
		r := &pb.AuthUserAddRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserAddRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserAdd = r
	case "AuthUserDelete":
		r := &pb.AuthUserDeleteRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserDeleteRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserDelete = r
	case "AuthUserGet":
		r := &pb.AuthUserGetRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserGetRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserGet = r
	case "AuthUserChangePassword":
		r := &pb.AuthUserChangePasswordRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserChangePasswordRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserChangePassword = r
	case "AuthUserGrantRole":
		r := &pb.AuthUserGrantRoleRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserGrantRoleRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserGrantRole = r
	case "AuthUserRevokeRole":
		r := &pb.AuthUserRevokeRoleRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserRevokeRoleRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserRevokeRole = r
	case "AuthUserList":
		r := &pb.AuthUserListRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthUserListRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthUserList = r
	case "AuthRoleList":
		r := &pb.AuthRoleListRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthRoleListRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthRoleList = r
	case "AuthRoleAdd":
		r := &pb.AuthRoleAddRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthRoleAddRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthRoleAdd = r
	case "AuthRoleDelete":
		r := &pb.AuthRoleDeleteRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthRoleDeleteRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthRoleDelete = r
	case "AuthRoleGet":
		r := &pb.AuthRoleGetRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthRoleGetRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthRoleGet = r
	case "AuthRoleGrantPermission":
		r := &pb.AuthRoleGrantPermissionRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthRoleGrantPermissionRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthRoleGrantPermission = r
	case "AuthRoleRevokePermission":
		r := &pb.AuthRoleRevokePermissionRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateAuthRoleRevokePermissionRequest(r)
		if err != nil {
			return err
		}
		internalRequest.AuthRoleRevokePermission = r
	case "ClusterVersionSet":
		r := &membershippb.ClusterVersionSetRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateClusterVersionSetRequest(r)
		if err != nil {
			return err
		}
		internalRequest.ClusterVersionSet = r
	case "ClusterMemberAttrSet":
		r := &membershippb.ClusterMemberAttrSetRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateClusterMemberAttrSetRequest(r)
		if err != nil {
			return err
		}
		internalRequest.ClusterMemberAttrSet = r
	case "DowngradeInfoSet":
		r := &membershippb.DowngradeInfoSetRequest{}
		err := f.GenerateStruct(r)
		if err != nil {
			return err
		}
		err = validateDowngradeInfoSetRequest(r)
		if err != nil {
			return err
		}
		internalRequest.DowngradeInfoSet = r
	}
	return nil
}

func init() {
	testing.Init()
	t := &testing.T{}
	lg := zaptest.NewLogger(t, zaptest.Level(zapcore.FatalLevel))

	cl := membership.NewCluster(lg)
	cl.SetStore(v2store.New())
	cl.AddMember(&membership.Member{ID: types.ID(1)}, true)

	be, _ := betesting.NewDefaultTmpBackend(t)
	//defer betesting.Close(t, be)

	schema.CreateMetaBucket(be.BatchTx())

	ci := cindex.NewConsistentIndex(be)
	tp, err := auth.NewTokenProvider(zap.NewExample(), tokenTypeSimple, dummyIndexWaiter, simpleTokenTTLDefault)
	if err != nil {
		panic(err)
	}

	srv := &EtcdServer{
		be:           be,
		lgMu:         new(sync.RWMutex),
		lg:           lg,
		id:           1,
		r:            *realisticRaftNode(lg),
		cluster:      cl,
		w:            wait.New(),
		consistIndex: ci,
		beHooks:      serverstorage.NewBackendHooks(lg, ci),
		authStore:    auth.NewAuthStore(zap.NewExample(), schema.NewAuthBackend(lg, be), tp, 0),
	}
	srv.kv = mvcc.New(zap.NewExample(), be, &lease.FakeLessor{}, mvcc.StoreConfig{})
	srv.lessor = &lease.FakeLessor{}
	alarmStore, err := v3alarm.NewAlarmStore(srv.lg, schema.NewAlarmBackend(srv.lg, srv.be))
	if err != nil {
		panic(err)
	}
	srv.alarmStore = alarmStore
	srv.be = be
	srv.applyV3Internal = srv.newApplierV3Internal()
	srv.applyV3 = srv.newApplierV3()
	ab = srv.newApplierV3Backend()
}

// Fuzzapply runs into panics that should not happen in production
// but that might happen when fuzzing. catchPanics() catches those
// panics.
func catchPanics() {
	if r := recover(); r != nil {
		var err string
		switch r.(type) {
		case string:
			err = r.(string)
		case runtime.Error:
			err = r.(runtime.Error).Error()
		case error:
			err = r.(error).Error()
		}
		if strings.Contains(err, "unknown entry type; must be either EntryNormal or EntryConfChange") {
			return
		} else if strings.Contains(err, "should never fail") {
			return
		} else if strings.Contains(err, "failed to unmarshal confChangeContext") {
			return
		} else if strings.Contains(err, "unknown ConfChange type") {
			return
		} else {
			panic(err)
		}
	}
}

// Fuzzapply tests func (s *EtcdServer).apply() with
// random entries.
func Fuzzapply(data []byte) int {
	defer catchPanics()

	f := fuzz.NewConsumer(data)

	// Create entries
	ents := make([]raftpb.Entry, 0)
	number, err := f.GetInt()
	if err != nil {
		return 0
	}
	for i := 0; i < number%20; i++ {
		ent := raftpb.Entry{}
		err = f.GenerateStruct(&ent)
		if err != nil {
			return 0
		}
		if len(ent.Data) == 0 {
			return 0
		}
		ents = append(ents, ent)
	}
	if len(ents) == 0 {
		return 0
	}

	// Setup server
	t := &testing.T{}
	lg := zaptest.NewLogger(t)

	cl := membership.NewCluster(zaptest.NewLogger(t))
	cl.SetStore(v2store.New())
	cl.AddMember(&membership.Member{ID: types.ID(1)}, true)

	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)

	schema.CreateMetaBucket(be.BatchTx())

	ci := cindex.NewConsistentIndex(be)
	srv := &EtcdServer{
		lgMu:         new(sync.RWMutex),
		lg:           lg,
		id:           1,
		r:            *realisticRaftNode(lg),
		cluster:      cl,
		w:            wait.New(),
		consistIndex: ci,
		beHooks:      serverstorage.NewBackendHooks(lg, ci),
	}

	// Pass entries to (s *EtcdServer).apply()
	_, _, _ = srv.apply(ents, &raftpb.ConfState{})
	return 1
}

func catchPanics2() {
	if r := recover(); r != nil {
		var err string
		switch r.(type) {
		case string:
			err = r.(string)
		case runtime.Error:
			err = r.(runtime.Error).Error()
		case error:
			err = r.(error).Error()
		}
		if strings.Contains(err, "is not in dotted-tri format") {
			return
		} else if strings.Contains(err, "strconv.ParseInt: parsing") {
			return
		} else if strings.Contains(err, "is not a valid semver identifier") {
			return
		} else if strings.Contains(err, "invalid downgrade; server version is lower than determined cluster version") {
			return
		} else if strings.Contains(err, "unexpected sort target") {
			return
		} else if strings.Contains(err, "failed to unmarshal 'authpb.User'") {
			return
		} else {
			panic(err)
		}
	}
}

func FuzzapplierV3backendApply(data []byte) int {
	defer catchPanics2()
	f := fuzz.NewConsumer(data)
	rr := &pb.InternalRaftRequest{}
	err := setRequestType(rr, f)
	if err != nil {
		return 0
	}
	//fmt.Printf("%+v\n", ab)
	_ = ab.Apply(rr, true)
	return 1
}
