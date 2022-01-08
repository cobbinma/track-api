use std::{collections::HashMap, sync::Arc};

use async_std::sync::RwLock;
use uuid::Uuid;

use crate::route::{NewRoute, Route, RouteStatus};

#[derive(Clone)]
pub struct Cache {
    routes: Arc<RwLock<HashMap<Uuid, Route>>>,
}

impl Cache {
    pub fn new() -> Self {
        Self {
            routes: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub async fn get_route(&self, id: Uuid) -> Result<Route, Box<dyn std::error::Error>> {
        self.routes
            .read()
            .await
            .get(&id)
            .cloned()
            .ok_or_else(|| "unable to find route".into())
    }

    pub async fn create_route(
        &self,
        new_route: NewRoute,
    ) -> Result<Route, Box<dyn std::error::Error>> {
        let id = Uuid::new_v4();
        let route = Route {
            id,
            user_id: new_route.user_id,
            status: RouteStatus::Active,
        };
        self.routes.write().await.insert(id, route.clone());

        Ok(route)
    }
}
