"""
MongoDB Atlas client for document storage.

Provides async MongoDB operations using Motor driver.
"""

from typing import Any, Dict, List, Optional

import structlog
from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorDatabase
from pymongo.errors import ConnectionFailure, ServerSelectionTimeoutError

from .config import get_settings

logger = structlog.get_logger(__name__)

# Global client instance
_client: Optional[AsyncIOMotorClient] = None
_database: Optional[AsyncIOMotorDatabase] = None


async def connect() -> AsyncIOMotorDatabase:
    """
    Connect to MongoDB Atlas.

    Returns:
        AsyncIOMotorDatabase: Database instance.

    Raises:
        ConnectionFailure: If connection fails.
    """
    global _client, _database

    if _database is not None:
        return _database

    settings = get_settings()

    try:
        _client = AsyncIOMotorClient(
            settings.mongodb_uri,
            serverSelectionTimeoutMS=5000,
            connectTimeoutMS=10000,
            maxPoolSize=50,
            minPoolSize=10,
        )

        # Verify connection
        await _client.admin.command("ping")

        _database = _client[settings.mongodb_database]

        logger.info(
            "Connected to MongoDB",
            database=settings.mongodb_database,
        )

        return _database

    except (ConnectionFailure, ServerSelectionTimeoutError) as e:
        logger.error("Failed to connect to MongoDB", error=str(e))
        raise


async def disconnect() -> None:
    """Close MongoDB connection."""
    global _client, _database

    if _client is not None:
        _client.close()
        _client = None
        _database = None
        logger.info("Disconnected from MongoDB")


async def health_check() -> bool:
    """
    Check MongoDB connection health.

    Returns:
        bool: True if healthy, False otherwise.
    """
    global _client

    if _client is None:
        return False

    try:
        await _client.admin.command("ping")
        return True
    except Exception as e:
        logger.error("MongoDB health check failed", error=str(e))
        return False


def get_database() -> Optional[AsyncIOMotorDatabase]:
    """
    Get the database instance.

    Returns:
        Optional[AsyncIOMotorDatabase]: Database instance or None.
    """
    return _database


class DocumentStore:
    """Document storage operations."""

    def __init__(self, collection_name: str):
        """
        Initialize document store.

        Args:
            collection_name: MongoDB collection name.
        """
        self.collection_name = collection_name

    @property
    def collection(self):
        """Get the collection instance."""
        db = get_database()
        if db is None:
            raise RuntimeError("Database not connected")
        return db[self.collection_name]

    async def insert(self, document: Dict[str, Any]) -> str:
        """
        Insert a document.

        Args:
            document: Document to insert.

        Returns:
            str: Inserted document ID.
        """
        result = await self.collection.insert_one(document)
        return str(result.inserted_id)

    async def find_by_id(self, doc_id: str) -> Optional[Dict[str, Any]]:
        """
        Find document by ID.

        Args:
            doc_id: Document ID.

        Returns:
            Optional[Dict[str, Any]]: Document or None.
        """
        from bson import ObjectId

        return await self.collection.find_one({"_id": ObjectId(doc_id)})

    async def find(
        self,
        query: Dict[str, Any],
        limit: int = 100,
        skip: int = 0,
    ) -> List[Dict[str, Any]]:
        """
        Find documents matching query.

        Args:
            query: Query filter.
            limit: Maximum documents to return.
            skip: Number of documents to skip.

        Returns:
            List[Dict[str, Any]]: List of matching documents.
        """
        cursor = self.collection.find(query).skip(skip).limit(limit)
        return await cursor.to_list(length=limit)

    async def update(
        self,
        doc_id: str,
        update: Dict[str, Any],
    ) -> bool:
        """
        Update a document.

        Args:
            doc_id: Document ID.
            update: Update operations.

        Returns:
            bool: True if document was modified.
        """
        from bson import ObjectId

        result = await self.collection.update_one(
            {"_id": ObjectId(doc_id)},
            {"$set": update},
        )
        return result.modified_count > 0

    async def delete(self, doc_id: str) -> bool:
        """
        Delete a document.

        Args:
            doc_id: Document ID.

        Returns:
            bool: True if document was deleted.
        """
        from bson import ObjectId

        result = await self.collection.delete_one({"_id": ObjectId(doc_id)})
        return result.deleted_count > 0


def get_document_store() -> DocumentStore:
    """
    Get document store instance.

    Returns:
        DocumentStore: Document store for the documents collection.
    """
    settings = get_settings()
    return DocumentStore(settings.mongodb_collection_documents)


def get_embeddings_store() -> DocumentStore:
    """
    Get embeddings store instance.

    Returns:
        DocumentStore: Document store for the embeddings collection.
    """
    settings = get_settings()
    return DocumentStore(settings.mongodb_collection_embeddings)
