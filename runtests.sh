virtualenv venv
source venv/bin/activate
pip install -r requirements.txt
go build
cd tests
sudo -E python test.py
