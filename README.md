# fediverse.express

We can make the fediverse huge, together. fediverse.express makes starting your own instance easy - deploy Misskey using just your browser.

## Build

You will need Go 1.16+, Ansible, and a DigitalOcean API application.

```
sudo apt install python3-pip
sudo python3 -m pip install ansible
ansible-galaxy collection install community.general community.postgresql

sudo snap install go --channel=1.16/stable --classic

git clone git@github.com:CuteAP/fediverse.express.git
cp .env.example .env
$EDITOR .env

go build && ./fediverse.express
```

## Hack

Please. If you would be so nice as to run your commits through gofmt before submitting them, that would be appreciated.

Also please include a Developer Certificate of Origin (`Signed-off-by` header).

## License

Copyright &copy; 2021 CuteAP/fediverse.express.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.