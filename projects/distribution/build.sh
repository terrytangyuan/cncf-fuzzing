#!/bin/bash -eu
set -o nounset
set -o pipefail
set -o errexit
set -x

export CNCFPATH="${SRC}/"cncf-fuzzing/projects/distribution
export DISTRIBUTION="github.com/distribution/distribution/v3"
export REGISTRYPATH="${DISTRIBUTION}/registry"

mv $CNCFPATH/inmemory_fuzzer.go $SRC/distribution/registry/storage/driver/inmemory/

mv $CNCFPATH/registry_fuzzer.go $SRC/distribution/registry/
mv $SRC/distribution/registry/registry_test.go \
   $SRC/distribution/registry/registry_test_fuzz.go

mv $CNCFPATH/client_fuzzer.go $SRC/distribution/registry/client/
mv $SRC/distribution/registry/client/repository_test.go \
   $SRC/distribution/registry/client/repository_test_fuzz.go

mv $CNCFPATH/storage_fuzzer.go $SRC/distribution/registry/storage/
mv $SRC/distribution/registry/storage/garbagecollect_test.go \
   $SRC/distribution/registry/storage/garbagecollect_test_fuzz.go


mv $CNCFPATH/access_controller_fuzzer.go $SRC/distribution/registry/auth/htpasswd/

mv $CNCFPATH/swift_fuzzer.go $SRC/distribution/registry/storage/driver/swift/

mv $CNCFPATH/s3_aws_fuzzer.go $SRC/distribution/registry/storage/driver/s3-aws/

mv $CNCFPATH/ocischema_fuzzer.go $SRC/distribution/manifest/ocischema/ 

mv $SRC/distribution/manifest/schema1/config_builder_test.go \
   $SRC/distribution/manifest/schema1/config_builder_test_fuzz.go
mv $CNCFPATH/schema1_fuzzer.go $SRC/distribution/manifest/schema1/

mv $CNCFPATH/authchallenge_fuzzer.go $SRC/distribution/registry/client/auth/challenge/
mv $CNCFPATH/token_fuzzer.go $SRC/distribution/registry/auth/token/
mv $CNCFPATH/set_fuzzer.go $SRC/distribution/digestset/
mv $CNCFPATH/reference_fuzzer2.go $SRC/distribution/reference/

go mod edit -dropreplace google.golang.org/grpc
go mod download && go mod tidy && go mod vendor

$SRC/distribution/script/oss_fuzz_build.sh

compile_go_fuzzer $DISTRIBUTION/reference FuzzParseNormalizedNamed fuzz_parse_normalized_named
compile_go_fuzzer $DISTRIBUTION/reference FuzzWithNameAndWithTag fuzz_with_name_and_tag
compile_go_fuzzer $DISTRIBUTION/manifest/ocischema FuzzManifestBuilder fuzz_manifest_builder

compile_go_fuzzer $REGISTRYPATH/auth/htpasswd FuzzAccessController fuzz_access_controller
compile_go_fuzzer $DISTRIBUTION/manifest/schema1 FuzzSchema1Build fuzz_schema1_build
compile_go_fuzzer $DISTRIBUTION/manifest/schema1 FuzzSchema1Verify fuzz_schema1_verify
compile_go_fuzzer $DISTRIBUTION/digestset FuzzSet fuzz_set
compile_go_fuzzer $REGISTRYPATH/auth/token FuzzToken fuzz_token
compile_go_fuzzer $REGISTRYPATH/auth/token FuzzToken2 fuzz_token2
compile_go_fuzzer $REGISTRYPATH/client/auth/challenge FuzzParseValueAndParams fuzz_parse_value_and_params
compile_go_fuzzer $REGISTRYPATH FuzzRegistry1 fuzz_registry1
compile_go_fuzzer $REGISTRYPATH FuzzRegistry2 fuzz_registry2
compile_go_fuzzer $REGISTRYPATH/client FuzzBlobServeBlob fuzz_blob_serve_blob
compile_go_fuzzer $REGISTRYPATH/client FuzzClientPut fuzz_client_put
compile_go_fuzzer $REGISTRYPATH/storage FuzzSchema2ManifestHandler fuzz_schema2_manifest_handler
compile_go_fuzzer $REGISTRYPATH/storage FuzzBlob fuzz_blob
compile_go_fuzzer $REGISTRYPATH/storage FuzzMarkAndSweep fuzz_mark_and_sweep
compile_go_fuzzer $REGISTRYPATH/storage FuzzFR fuzz_fr
compile_go_fuzzer $REGISTRYPATH/storage/driver/inmemory FuzzInmemoryDriver fuzz_inmemory_driver
compile_go_fuzzer $REGISTRYPATH/storage/driver/s3-aws FuzzS3Driver fuzz_s3_driver
compile_go_fuzzer $REGISTRYPATH/storage/driver/swift FuzzSwift fuzz_swift
