set -o nounset
set -o pipefail
set -o errexit
set -x

# Delete the "FORBIDDEN_DEPENDENCY" replacements
sed -i '/FORBIDDEN_DEPENDENCY/d' $SRC/etcd/server/go.mod
sed -i '/FORBIDDEN_DEPENDENCY/d' $SRC/etcd/raft/go.mod

# Change panic message so we can catch them:
sed 's/panic(err)/panic("GOT A FUZZ ERROR")/g' -i $SRC/etcd/raft/raft.go


mkdir $SRC/etcd/tests/fuzzing

# api marshal fuzzer
cd $SRC/etcd
mv $SRC/cncf-fuzzing/projects/etcd/autogenerate_api_marshal_fuzzer.go ./
grep -r ") Marshal(" .>>"/tmp/marshal_targets.txt"
go run autogenerate_api_marshal_fuzzer.go
rm autogenerate_api_marshal_fuzzer.go
mv api_marshal_fuzzer.go ./tests/fuzzing/
cd tests/fuzzing
compile_go_fuzzer go.etcd.io/etcd/tests/v3/fuzzing FuzzAPIMarshal fuzz_api_marshal

# wal fuzzer
echo "building wal fuzzer"
cp $SRC/cncf-fuzzing/projects/etcd/wal_fuzzer.go $SRC/etcd/server/storage/wal/
compile_go_fuzzer go.etcd.io/etcd/server/v3/storage/wal FuzzWalCreate fuzz_wal_create
compile_go_fuzzer go.etcd.io/etcd/server/v3/storage/wal FuzzMinimalEtcdVersion fuzz_minimal_etcd_version

# grpc proxy fuzzer
echo "building grpc proxy fuzzer"
mv $SRC/cncf-fuzzing/projects/etcd/grpc_proxy_fuzzer.go $SRC/etcd/tests/fuzzing/
cd $SRC/etcd/tests/fuzzing
go mod tidy
sed -i '88 a return' $SRC/etcd/client/pkg/testutil/testutil.go
compile_go_fuzzer go.etcd.io/etcd/tests/v3/fuzzing FuzzKVProxy fuzz_kv_proxy

# grpc api fuzzer
mv $SRC/cncf-fuzzing/projects/etcd/v3_grpc_fuzzer.go $SRC/etcd/tests/fuzzing/
cd $SRC/etcd/tests/fuzzing
compile_go_fuzzer go.etcd.io/etcd/tests/v3/fuzzing FuzzGRPCApis fuzz_grpc_apis
cd -

# snapshot fuzzer
# timeouts during coverage build
if [ "$SANITIZER" != "coverage" ]
then
	cd $SRC/etcd/server/etcdserver/api/snap
	mv $SRC/cncf-fuzzing/projects/etcd/snapshot_fuzzer.go ./
	compile_go_fuzzer go.etcd.io/etcd/server/v3/etcdserver/api/snap FuzzSnapLoad fuzz_snap_load
fi

# mvcc fuzzer
cd $SRC/etcd/server/storage/mvcc
mv $SRC/cncf-fuzzing/projects/etcd/mvcc_fuzzer.go ./
go get github.com/AdaLogics/go-fuzz-headers
mv kv_test.go kv_test_fuzz.go
mv kvstore_test.go kvstore_test_fuzz.go
# disable some logging:
sed -i '/s.lg.Info("kvstore restored"/c\\/\/s.lg.Info("kvstore restored"' $SRC/etcd/server/storage/mvcc/kvstore.go
compile_go_fuzzer go.etcd.io/etcd/server/v3/storage/mvcc FuzzMvccStorage fuzz_mvcc_storage
compile_go_fuzzer go.etcd.io/etcd/server/v3/storage/mvcc FuzzMvccIndex fuzz_mvcc_index

# proxy fuzzer
cd $SRC/etcd/pkg/proxy
mv server_test.go server_test_fuzz.go
mv $SRC/cncf-fuzzing/projects/etcd/proxy_fuzzer.go ./
compile_go_fuzzer go.etcd.io/etcd/pkg/v3/proxy FuzzProxyServer fuzz_proxy_server

# etcdserver fuzzer
echo "building etcdserver fuzzer"
cd $SRC/etcd/server/etcdserver
go get github.com/AdaLogics/go-fuzz-headers
mv server_test.go server_test_fuzz.go
mv $SRC/cncf-fuzzing/projects/etcd/etcdserver_fuzzer.go ./
compile_go_fuzzer go.etcd.io/etcd/server/v3/etcdserver Fuzzapply fuzz_etcdserver_apply
compile_go_fuzzer go.etcd.io/etcd/server/v3/etcdserver FuzzapplierV3backendApply fuzz_applier_v3_backend_apply

# auth store fuzzer
echo "building auth fuzzer"
cd $SRC/etcd/server/auth
go get github.com/AdaLogics/go-fuzz-headers
mv $SRC/cncf-fuzzing/projects/etcd/auth_store_fuzzer.go ./
mv store_mock_test.go store_mock_test_fuzz.go
mv store_test.go store_test_fuzz.go
mv jwt_test.go jwt_test_fuzz.go
compile_go_fuzzer go.etcd.io/etcd/server/v3/auth FuzzAuthStore fuzz_auth_store

# backend fuzzer
echo "building backend fuzzer"
cd $SRC/etcd/server/storage/backend/testing
mv $SRC/cncf-fuzzing/projects/etcd/backend_fuzzer.go ./
go get github.com/AdaLogics/go-fuzz-headers
compile_go_fuzzer go.etcd.io/etcd/server/v3/storage/backend/testing FuzzBackend fuzz_backend


# rafthttp fuzzer
mv $SRC/cncf-fuzzing/projects/etcd/raft_api_fuzzer.go $SRC/etcd/server/etcdserver/api/rafthttp/
mv $SRC/etcd/server/etcdserver/api/rafthttp/functional_test.go \
   $SRC/etcd/server/etcdserver/api/rafthttp/functional_test_fuzz.go

cd $SRC/etcd/server/etcdserver/api/rafthttp
go mod tidy
compile_go_fuzzer go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp FuzzRaftHttpRequests fuzz_raft_http_requests

compile_go_fuzzer go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp FuzzMessageEncodeDecode fuzz_message_encode_decode

# raft fuzzer
cd $SRC/etcd/raft
mv $SRC/cncf-fuzzing/projects/etcd/raft_fuzzer.go ./
go mod tidy
mv diff_test.go diff_test_fuzz.go
mv log_test.go log_test_fuzz.go
mv raft_test.go raft_test_fuzz.go
compile_go_fuzzer go.etcd.io/etcd/raft/v3 FuzzStep fuzz_step

# file_purge_fuzzer
cd $SRC/etcd/client/pkg/fileutil
go get github.com/AdaLogics/go-fuzz-headers
mv $SRC/cncf-fuzzing/projects/etcd/filepurge_fuzzer.go $SRC/etcd/client/pkg/fileutil/
compile_go_fuzzer go.etcd.io/etcd/client/pkg/v3/fileutil FuzzPurgeFile fuzz_purge_file
cd -
