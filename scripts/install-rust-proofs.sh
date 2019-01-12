#!/usr/bin/env bash

install_precompiled() {
  RELEASE_SHA1=`git rev-parse @:./proofs/rust-proofs`
  RELEASE_NAME="rust-proofs-`uname`"
  RELEASE_TAG="${RELEASE_SHA1:0:16}"

  if [ -z $GITHUB_TOKEN ]; then
    echo "\$GITHUB_TOKEN not set"
    return 1
  fi

  RELEASE_RESPONSE=`
    curl \
      --header "Authorization: token $GITHUB_TOKEN" \
      "https://api.github.com/repos/filecoin-project/rust-proofs/releases/tags/$RELEASE_TAG"
  `

  RELEASE_ID=`echo $RELEASE_RESPONSE | jq '.id'`

  if [ "$RELEASE_ID" == "null" ]; then
    echo "release does not exist"
    return 1
  fi

  RELEASE_URL=`echo $RELEASE_RESPONSE | jq -r ".assets[] | select(.name | contains(\"$RELEASE_NAME\")) | .url"`

  ASSET_URL=`curl \
      --header "Authorization: token $GITHUB_TOKEN" \
      --header "Accept:application/octet-stream" \
      --location \
      --output /dev/null \
      -w %{url_effective} \
      "$RELEASE_URL"
  `

  curl --output /tmp/$RELEASE_NAME.tar.gz "$ASSET_URL"

  if [ $? -ne "0" ]; then
    echo "asset failed to be downloaded"
    return 1
  fi

  tar -C /usr/local -xzf /tmp/$RELEASE_NAME.tar.gz
}

install_local() {
  git submodule update --init --recursive

  pushd proofs/rust-proofs

  cargo --version
  cargo update
  cargo build --release --all

  popd

  mkdir -p /usr/local/lib/pkgconfig

  cp proofs/rust-proofs/target/release/paramcache /usr/local/bin/
  cp proofs/rust-proofs/target/release/libfilecoin_proofs.h /usr/local/include/
  cp proofs/rust-proofs/target/release/libfilecoin_proofs.a /usr/local/lib/
  cp proofs/rust-proofs/target/release/libfilecoin_proofs.pc /usr/local/lib/pkgconfig/
}

if [ -z "$FILECOIN_USE_PRECOMPILED_RUST_PROOFS" ]; then
  echo "using local rust-proofs"
  install_local
else
  echo "using precompiled rust-proofs"
  install_precompiled

  if [ $? -ne "0" ]; then
    echo "failed to find or obtain precompiled rust-proofs, falling back to local"
    install_local
  fi
fi
