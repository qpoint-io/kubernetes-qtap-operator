#!/usr/bin/env sh

# certificate authority bundles for common distributions
bundles="
/etc/ssl/certs/ca-certificates.crt \
/etc/pki/tls/certs/ca-bundle.crt \
/etc/ssl/ca-bundle.pem \
/etc/pki/tls/cacert.pem \
/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem \
/etc/ssl/cert.pem
"

# qpoint ca
qpoint_ca="/mnt/ca/qpoint.pem"

# tls config destination
destination="/mnt/tls"

# initialize a flag to check if config was copied
config_copied="no"

# convert string to an array-like structure
set -- $bundles

# iterate over the possible bundles
for bundle; do
  if [ -f "$bundle" ]; then
    echo "Found ca bundle: ${bundle}"

    # extract the directly/file from the bundle
    directory=$(dirname "$bundle")
    file=$(basename "$bundle")

    # copy the contents of the bundle into the shared mount
    echo "Copying contents of $directory to $destination"
    cp -R "$directory"/* "$destination"

    # append the qpoint root ca
    cat "$qpoint_ca" >> "$destination/$file"

    # leave the breadcrumb
    config_copied="yes"

    # stop after the first found bundle
    break
  fi
done

# we didn't find a matching config, likely there is no openssl installed
# 
# in this case our approach is to just add our ca to all of the bundles
# for the potential distributions
if [ "$config_copied" = "no" ]; then
  echo "No config copied, creating new CA bundles."
  set -- $bundles
  for bundle; do
    file=$(basename "$bundle")
    cat "$qpoint_ca" > "$destination/$file"
  done
fi
