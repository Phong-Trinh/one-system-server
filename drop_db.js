const { MongoClient } = require('mongodb');

async function run() {
  const uri = "mongodb://localhost:27017";
  const client = new MongoClient(uri);
  try {
    await client.connect();
    const db = client.db('one_system');
    await db.dropDatabase();
    console.log("Database 'one_system' dropped successfully.");
  } finally {
    await client.close();
  }
}

run().catch(console.dir);
