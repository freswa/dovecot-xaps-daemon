#!/usr/bin/env bash
# using read command without any variable
# change to "read -sp" if password input should be hidden

configparam="$1"
for configfile in  "$configparam" "/etc/xapsd.yaml" "/usr/local/etc/xapsd/xapsd.yaml" ;
do
    [[ "$configfile" == ""  ]] && continue
    [[ -f "$configfile" ]] && break
done
if [[ ! -f "$configfile" ]]
then
    echo && echo "ERROR:"
    [[ "$configparam" == "" ]] && echo "Please pass location of xapsd.yaml as first paramater."
    echo "ConfigFile '$configparam' not found. Exiting script"
    exit 1
fi

echo "Using config file: ./$configfile"

read -p 'Username: ' uservar
read -p 'Password: ' passvar 
[[ "$uservar" == "" ]] && echo "No username set. Exiting script" && exit 1
[[ "$passvar" == "" ]] && echo "No password set. Exiting script" && exit 1
hashvalue=`echo -n "$passvar" | openssl dgst -sha256 -binary | xxd -p -c 32`

sed -E -i "" "s/^(appleId[[:blank:]]*[=|:][[:blank:]]*).*/\1$uservar/" "$configfile"
sed -E -i "" "s/^(appleIdHashedPassword[[:blank:]]*[=|:][[:blank:]]*).*/\1$hashvalue/" "$configfile"

echo
echo "Seting new parameter in $configfile"
echo "appleId: $uservar"
echo "appleIdHashedPassword: $hashvalue"