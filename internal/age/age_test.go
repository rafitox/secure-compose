package age

import (
	"os"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	testData := "DATABASE_URL=postgres://user:secret@localhost/db\nAPI_KEY=sk_test_123"

	// Encrypt
	err := EncryptContent("/tmp/test_encrypt.age", []byte(testData), "my-secret-pass")
	if err != nil {
		t.Fatalf("EncryptContent failed: %v", err)
	}
	defer os.Remove("/tmp/test_encrypt.age")

	// Verify file was created
	if _, err := os.Stat("/tmp/test_encrypt.age"); os.IsNotExist(err) {
		t.Fatal("Encrypted file was not created")
	}

	// Decrypt with correct passphrase
	content, err := DecryptContent("/tmp/test_encrypt.age", "my-secret-pass")
	if err != nil {
		t.Fatalf("DecryptContent failed: %v", err)
	}

	if string(content) != testData {
		t.Errorf("Decrypted content doesn't match.\nExpected: %q\nGot: %q", testData, string(content))
	}

	// Decrypt with wrong passphrase should fail
	_, err = DecryptContent("/tmp/test_encrypt.age", "wrong-pass")
	if err == nil {
		t.Error("Expected error with wrong passphrase, got nil")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	// Create temp input file
	inputData := []byte("SECRET=value\nPASSWORD=hunter2")
	tmpInput, err := os.CreateTemp("", "test_input_*.env")
	if err != nil {
		t.Fatalf("Failed to create temp input: %v", err)
	}
	defer os.Remove(tmpInput.Name())
	tmpInput.Write(inputData)
	tmpInput.Close()

	tmpOutput := tmpInput.Name() + ".age"
	tmpDecrypted := tmpInput.Name() + ".decrypted"
	defer os.Remove(tmpOutput)
	defer os.Remove(tmpDecrypted)

	// Encrypt file
	err = EncryptFile(tmpInput.Name(), tmpOutput, "test-pass-123")
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Decrypt file
	err = DecryptFile(tmpOutput, tmpDecrypted, "test-pass-123")
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	// Verify decrypted content
	decrypted, err := os.ReadFile(tmpDecrypted)
	if err != nil {
		t.Fatalf("Failed to read decrypted file: %v", err)
	}

	if string(decrypted) != string(inputData) {
		t.Errorf("Decrypted content mismatch")
	}
}
